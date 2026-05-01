package steps_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"runloop/internal/steps"
	_ "runloop/internal/steps/shell"
	_ "runloop/internal/steps/transform"
	_ "runloop/internal/steps/wait"
	"runloop/internal/workflows"
)

type fakeSecrets struct {
	values map[string]string
}

func (s fakeSecrets) Resolve(ctx context.Context, id string) (string, error) {
	return s.values[id], nil
}

func (s fakeSecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return s.values[profile+"."+name], nil
}

func TestExecutorFailsWhenTransformInputTemplateMissing(t *testing.T) {
	executor := steps.NewExecutor()

	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:    "normalize",
		Type:  "transform",
		Input: map[string]any{"message": "{{ inbox.normalized.message }}"},
	}, workflows.Workflow{}, map[string]any{
		"inbox": map[string]any{"normalized": map[string]any{}},
	})

	if result.OK {
		t.Fatalf("expected failed result, got %#v", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Message, "inbox.normalized.message") {
		t.Fatalf("expected missing path in error, got %#v", result.Error)
	}
}

func TestWaitStepHonorsDurationAndRejectsInvalidDuration(t *testing.T) {
	executor := steps.NewExecutor()

	start := time.Now()
	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:       "pause",
		Type:     "wait",
		Duration: "5ms",
	}, workflows.Workflow{}, nil)
	if !result.OK {
		t.Fatalf("expected wait success, got %#v", result)
	}
	if elapsed := time.Since(start); elapsed < 5*time.Millisecond {
		t.Fatalf("wait returned too early after %s", elapsed)
	}
	if result.Data["waited"] != "5ms" {
		t.Fatalf("waited = %v", result.Data["waited"])
	}

	_, result = executor.Execute(context.Background(), workflows.Step{
		ID:       "pause",
		Type:     "wait",
		Duration: "not-a-duration",
	}, workflows.Workflow{}, nil)
	if result.OK || result.Error == nil {
		t.Fatalf("expected invalid wait duration failure, got %#v", result)
	}
}

func TestShellStepRequiresPermission(t *testing.T) {
	executor := steps.NewExecutor()

	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:      "cmd",
		Type:    "shell",
		Command: "printf no",
	}, workflows.Workflow{}, nil)

	if result.OK {
		t.Fatalf("expected shell permission failure, got %#v", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Message, "permissions.shell") {
		t.Fatalf("expected permission error, got %#v", result.Error)
	}
}

func TestShellStepCapturesStdoutStderrExitCodeAndArtifacts(t *testing.T) {
	executor := steps.NewExecutor()

	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:      "cmd",
		Type:    "shell",
		Command: "printf out; printf err >&2; exit 7",
	}, workflows.Workflow{Permissions: workflows.Permissions{Shell: true}}, nil)

	if result.OK {
		t.Fatalf("expected non-zero shell command to fail, got %#v", result)
	}
	if result.Data["stdout"] != "out" {
		t.Fatalf("stdout = %v", result.Data["stdout"])
	}
	if result.Data["stderr"] != "err" {
		t.Fatalf("stderr = %v", result.Data["stderr"])
	}
	if result.Data["exitCode"] != 7 {
		t.Fatalf("exitCode = %v", result.Data["exitCode"])
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("expected stdout/stderr artifacts, got %#v", result.Artifacts)
	}
	want := map[string]string{"stdout.log": "out", "stderr.log": "err"}
	for _, artifact := range result.Artifacts {
		if got, ok := want[artifact.Path]; !ok || got != artifact.Content {
			t.Fatalf("unexpected artifact %#v", artifact)
		}
	}
}

func TestShellStepReportsTimeout(t *testing.T) {
	executor := steps.NewExecutor()

	start := time.Now()
	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:      "cmd",
		Type:    "shell",
		Command: "sleep 1",
		Timeout: "10ms",
	}, workflows.Workflow{Permissions: workflows.Permissions{Shell: true}}, nil)
	elapsed := time.Since(start)

	if result.OK {
		t.Fatalf("expected shell timeout failure, got %#v", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Message, "timed out") {
		t.Fatalf("expected timeout error, got %#v", result.Error)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("timeout returned too slowly after %s", elapsed)
	}
}

func TestShellStepUsesMinimalAndExplicitEnv(t *testing.T) {
	t.Setenv("RUNLOOP_DAEMON_SECRET", "leak")
	executor := steps.NewExecutor(steps.WithBaseEnv(func() []string {
		return []string{"PATH=" + getenvForTest(t, "PATH")}
	}))

	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:      "cmd",
		Type:    "shell",
		Command: `printf "%s:%s" "${RUNLOOP_DAEMON_SECRET:-missing}" "$VISIBLE"`,
		Env: map[string]workflows.EnvValue{
			"VISIBLE": {Kind: workflows.EnvLiteral, Literal: "ok"},
		},
	}, workflows.Workflow{Permissions: workflows.Permissions{Shell: true}}, nil)

	if !result.OK {
		t.Fatalf("expected shell success, got %#v", result)
	}
	if result.Data["stdout"] != "missing:ok" {
		t.Fatalf("stdout = %q", result.Data["stdout"])
	}
}

func TestShellStepDefaultsToRunloopWorkspaceWorkdir(t *testing.T) {
	workspace := t.TempDir()
	executor := steps.NewExecutor()

	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:      "cmd",
		Type:    "shell",
		Command: "pwd",
	}, workflows.Workflow{Permissions: workflows.Permissions{Shell: true}}, map[string]any{
		"runloop": map[string]any{"workspace": workspace},
	})

	if !result.OK {
		t.Fatalf("expected shell success, got %#v", result)
	}
	if strings.TrimSpace(result.Data["stdout"].(string)) != workspace {
		t.Fatalf("stdout = %q", result.Data["stdout"])
	}
}

func TestShellStepResolvesSecretAndProfileEnv(t *testing.T) {
	executor := steps.NewExecutor(
		steps.WithSecrets(fakeSecrets{values: map[string]string{
			"token":                    "direct",
			"claude.ANTHROPIC_API_KEY": "profiled",
		}}),
		steps.WithBaseEnv(func() []string { return []string{"PATH=" + getenvForTest(t, "PATH")} }),
	)

	_, result := executor.Execute(context.Background(), workflows.Step{
		ID:      "cmd",
		Type:    "shell",
		Command: `printf "%s:%s" "$DIRECT" "$PROFILED"`,
		Env: map[string]workflows.EnvValue{
			"DIRECT":   {Kind: workflows.EnvSecret, Secret: "token"},
			"PROFILED": {Kind: workflows.EnvFromProfile, From: "claude.ANTHROPIC_API_KEY"},
		},
	}, workflows.Workflow{Permissions: workflows.Permissions{Shell: true}}, nil)

	if !result.OK {
		t.Fatalf("expected shell success, got %#v", result)
	}
	if result.Data["stdout"] != "direct:profiled" {
		t.Fatalf("stdout = %q", result.Data["stdout"])
	}
}

func getenvForTest(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s must be set for shell tests", key)
	}
	return value
}
