package claude

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"runloop/internal/secrets"
	"runloop/internal/steps"
	"runloop/internal/workflows"
)

type fakeRunner struct {
	command string
	args    []string
	env     []string
	dir     string
}

func (r *fakeRunner) Run(ctx context.Context, command string, args []string, env []string, dir string) (Output, error) {
	r.command = command
	r.args = args
	r.env = env
	r.dir = dir
	return Output{Stdout: "done", Stderr: "note", ExitCode: 0}, nil
}

func TestClaudeStepBuildsCLIArgsAndUsesWorkspace(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step: workflows.Step{
			ID:             "agent",
			Type:           "claude",
			Prompt:         "Summarize {{ input.message }}",
			Model:          "sonnet",
			PermissionMode: "plan",
		},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		Input:    map[string]any{"message": "hello"},
		StepCtx: map[string]any{
			"input":   map[string]any{"message": "hello"},
			"runloop": map[string]any{"workspace": t.TempDir()},
		},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if runner.command != "claude" {
		t.Fatalf("command = %q", runner.command)
	}
	wantArgs := []string{"-p", "--permission-mode", "plan", "--model", "sonnet", "Summarize hello"}
	if len(runner.args) != len(wantArgs) {
		t.Fatalf("args = %#v", runner.args)
	}
	for i := range wantArgs {
		if runner.args[i] != wantArgs[i] {
			t.Fatalf("args[%d] = %q, want %q; all args %#v", i, runner.args[i], wantArgs[i], runner.args)
		}
	}
	if runner.dir == "" {
		t.Fatal("expected workdir")
	}
}

type fakeSecrets struct {
	values     map[string]string
	connValues map[string]string
	connErrs   map[string]error
}

func (s fakeSecrets) Resolve(ctx context.Context, id string) (string, error) {
	return s.values[id], nil
}

func (s fakeSecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return s.values[profile+"."+name], nil
}

func (s fakeSecrets) ResolveConnectionEnv(ctx context.Context, ref string, name string) (string, error) {
	key := ref + "." + name
	if err := s.connErrs[key]; err != nil {
		return "", err
	}
	return s.connValues[key], nil
}

func (s fakeSecrets) ConnectionConfigured(ref string) bool {
	return true
}

func (s fakeSecrets) ListConnections() []secrets.Connection {
	return nil
}

func (s fakeSecrets) TestConnection(ctx context.Context, ref string) error {
	return nil
}

func TestClaudeStepAutoInjectsConfiguredProfileEnv(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step:     workflows.Step{ID: "agent", Type: "claude", Prompt: "hello"},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{values: map[string]string{
			"claude.ANTHROPIC_API_KEY": "sk-test",
		}},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if !containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-test") {
		t.Fatalf("env = %#v", runner.env)
	}
}

func TestClaudeStepAutoWithConnectionInjectsConnectionAPIKey(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step: workflows.Step{
			ID:         "agent",
			Type:       "claude",
			Prompt:     "hello",
			Auth:       "auto",
			Connection: "claude.default",
		},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{
			values: map[string]string{
				"claude.ANTHROPIC_API_KEY": "sk-profile",
			},
			connValues: map[string]string{
				"claude.default.ANTHROPIC_API_KEY": "sk-connection",
			},
		},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if !containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-connection") {
		t.Fatalf("env = %#v", runner.env)
	}
}

func TestClaudeStepDefaultAuthWithConnectionInjectsConnectionAPIKey(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step: workflows.Step{
			ID:         "agent",
			Type:       "claude",
			Prompt:     "hello",
			Connection: "claude.default",
		},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{
			values: map[string]string{
				"claude.ANTHROPIC_API_KEY": "sk-profile",
			},
			connValues: map[string]string{
				"claude.default.ANTHROPIC_API_KEY": "sk-connection",
			},
		},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if !containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-connection") {
		t.Fatalf("env = %#v", runner.env)
	}
	if containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-profile") {
		t.Fatalf("profile API key should not be used when connection is set, env = %#v", runner.env)
	}
}

func TestClaudeStepAPIKeyWithoutConnectionInjectsProfileAPIKey(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step: workflows.Step{
			ID:     "agent",
			Type:   "claude",
			Prompt: "hello",
			Auth:   "apiKey",
		},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{
			values: map[string]string{
				"claude.ANTHROPIC_API_KEY": "sk-profile",
			},
			connValues: map[string]string{
				"claude.default.ANTHROPIC_API_KEY": "sk-connection",
			},
		},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if !containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-profile") {
		t.Fatalf("env = %#v", runner.env)
	}
	if containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-connection") {
		t.Fatalf("connection API key should not be used without connection, env = %#v", runner.env)
	}
}

func TestClaudeStepAPIKeyWithConnectionPrefersConnectionOverProfile(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step: workflows.Step{
			ID:         "agent",
			Type:       "claude",
			Prompt:     "hello",
			Auth:       "apiKey",
			Connection: "claude.default",
		},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{
			values: map[string]string{
				"claude.ANTHROPIC_API_KEY": "sk-profile",
			},
			connValues: map[string]string{
				"claude.default.ANTHROPIC_API_KEY": "sk-connection",
			},
		},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if !containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-connection") {
		t.Fatalf("env = %#v", runner.env)
	}
	if containsEnv(runner.env, "ANTHROPIC_API_KEY=sk-profile") {
		t.Fatalf("profile API key should not be used when connection is set, env = %#v", runner.env)
	}
}

func TestClaudeStepLoginAuthDoesNotInjectAPIKey(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step:     workflows.Step{ID: "agent", Type: "claude", Prompt: "hello", Auth: "login"},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{values: map[string]string{
			"claude.ANTHROPIC_API_KEY": "sk-test",
		}},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	for _, item := range runner.env {
		if strings.HasPrefix(item, "ANTHROPIC_API_KEY=") {
			t.Fatalf("login auth should not inject API key, env = %#v", runner.env)
		}
	}
}

func TestClaudeStepLoginAuthIgnoresConnection(t *testing.T) {
	runner := &fakeRunner{}
	oldRunner := commandRunner
	commandRunner = runner
	defer func() { commandRunner = oldRunner }()

	_, result := Execute(context.Background(), steps.Request{
		Step: workflows.Step{
			ID:         "agent",
			Type:       "claude",
			Prompt:     "hello",
			Auth:       "login",
			Connection: "claude.default",
		},
		Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
		StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
		Secrets: fakeSecrets{
			connErrs: map[string]error{
				"claude.default.ANTHROPIC_API_KEY": fmt.Errorf("broken connection"),
			},
		},
	})
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	for _, item := range runner.env {
		if strings.HasPrefix(item, "ANTHROPIC_API_KEY=") {
			t.Fatalf("login auth should not inject API key, env = %#v", runner.env)
		}
	}
}

func TestClaudeStepConnectionRequiresConnectionEnvSupportForAutoAndAPIKey(t *testing.T) {
	for _, auth := range []string{"auto", "apiKey"} {
		t.Run(auth, func(t *testing.T) {
			_, result := Execute(context.Background(), steps.Request{
				Step: workflows.Step{
					ID:         "agent",
					Type:       "claude",
					Prompt:     "hello",
					Auth:       auth,
					Connection: "claude.default",
				},
				Workflow: workflows.Workflow{Permissions: workflows.Permissions{Shell: true}},
				StepCtx:  map[string]any{"runloop": map[string]any{"workspace": t.TempDir()}},
				Secrets: fakeProfileOnlySecrets{values: map[string]string{
					"claude.ANTHROPIC_API_KEY": "sk-profile",
				}},
			})
			if result.OK || result.Error == nil {
				t.Fatalf("result = %#v", result)
			}
			if !strings.Contains(result.Error.Message, "claude.default") || !strings.Contains(result.Error.Message, "ANTHROPIC_API_KEY") {
				t.Fatalf("error = %q", result.Error.Message)
			}
		})
	}
}

type fakeProfileOnlySecrets struct {
	values map[string]string
}

func (s fakeProfileOnlySecrets) Resolve(ctx context.Context, id string) (string, error) {
	return s.values[id], nil
}

func (s fakeProfileOnlySecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return s.values[profile+"."+name], nil
}

func containsEnv(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}
