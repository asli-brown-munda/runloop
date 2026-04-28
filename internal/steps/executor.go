package steps

import (
	"context"
	"fmt"
	"time"

	"runloop/internal/workflows"
)

type Executor struct{}

func NewExecutor() *Executor { return &Executor{} }

func (e *Executor) Execute(ctx context.Context, step workflows.Step, wf workflows.Workflow, baseCtx map[string]any) (map[string]any, StepResult) {
	input := RenderMap(step.Input, baseCtx)
	stepCtx := cloneMap(baseCtx)
	stepCtx["input"] = input
	switch step.Type {
	case "transform":
		return input, StepResult{OK: true, Data: RenderMap(step.Output, stepCtx)}
	case "wait":
		if step.Duration != "" {
			duration, err := time.ParseDuration(step.Duration)
			if err != nil {
				return input, StepResult{OK: false, Error: &StepError{Message: err.Error()}}
			}
			timer := time.NewTimer(duration)
			select {
			case <-ctx.Done():
				timer.Stop()
				return input, StepResult{OK: false, Error: &StepError{Message: ctx.Err().Error()}}
			case <-timer.C:
			}
		}
		return input, StepResult{OK: true, Data: map[string]any{"waited": step.Duration}}
	case "shell":
		if !wf.Permissions.Shell {
			return input, StepResult{OK: false, Error: &StepError{Message: "shell steps are disabled unless workflow permissions.shell is true"}}
		}
		return runShell(ctx, step, input)
	default:
		return input, StepResult{OK: false, Error: &StepError{Message: fmt.Sprintf("unsupported step type %q", step.Type)}}
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}
