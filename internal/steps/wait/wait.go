package wait

import (
	"context"
	"time"

	"runloop/internal/steps"
)

func init() { steps.Register("wait", Execute) }

// Durable wait/resume support is intentionally deferred beyond the MVP.
func Execute(ctx context.Context, req steps.Request) (map[string]any, steps.StepResult) {
	step := req.Step
	input := req.Input
	if step.Duration == "" {
		return input, steps.StepResult{OK: true, Data: map[string]any{"waited": step.Duration}}
	}
	duration, err := time.ParseDuration(step.Duration)
	if err != nil {
		return input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return input, steps.StepResult{OK: false, Error: &steps.StepError{Message: ctx.Err().Error()}}
	case <-timer.C:
		return input, steps.StepResult{OK: true, Data: map[string]any{"waited": step.Duration}}
	}
}
