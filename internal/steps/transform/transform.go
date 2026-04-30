package transform

import (
	"context"

	"runloop/internal/steps"
)

func init() { steps.Register("transform", Execute) }

func Execute(ctx context.Context, req steps.Request) (map[string]any, steps.StepResult) {
	output, err := steps.RenderMap(req.Step.Output, req.StepCtx)
	if err != nil {
		return req.Input, steps.StepResult{OK: false, Error: &steps.StepError{Message: err.Error()}}
	}
	return req.Input, steps.StepResult{OK: true, Data: output}
}
