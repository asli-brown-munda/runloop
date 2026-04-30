package steps_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"runloop/internal/steps"
	_ "runloop/internal/steps/shell"
	_ "runloop/internal/steps/transform"
	_ "runloop/internal/steps/wait"
	"runloop/internal/workflows"
)

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
