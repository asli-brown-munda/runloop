package claude

import (
	"context"
	"strings"
	"testing"

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
	values map[string]string
}

func (s fakeSecrets) Resolve(ctx context.Context, id string) (string, error) {
	return s.values[id], nil
}

func (s fakeSecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return s.values[profile+"."+name], nil
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

func containsEnv(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}
