package steps

import (
	"context"
	"os/exec"
	"time"

	"runloop/internal/workflows"
)

func runShell(ctx context.Context, step workflows.Step, input map[string]any) (map[string]any, StepResult) {
	timeout := 30 * time.Second
	if step.Timeout != "" {
		parsed, err := time.ParseDuration(step.Timeout)
		if err != nil {
			return input, StepResult{OK: false, Error: &StepError{Message: err.Error()}}
		}
		timeout = parsed
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "sh", "-c", step.Command)
	stdout, err := cmd.CombinedOutput()
	data := map[string]any{"output": string(stdout)}
	if err != nil {
		return input, StepResult{OK: false, Data: data, Error: &StepError{Message: err.Error()}}
	}
	return input, StepResult{OK: true, Data: data}
}
