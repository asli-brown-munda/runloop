package steps

import (
	"context"
	"fmt"

	"runloop/internal/workflows"
)

type Executor struct{}

func NewExecutor() *Executor { return &Executor{} }

func (e *Executor) Execute(ctx context.Context, step workflows.Step, wf workflows.Workflow, baseCtx map[string]any) (map[string]any, StepResult) {
	input, err := RenderMap(step.Input, baseCtx)
	if err != nil {
		return input, StepResult{OK: false, Error: &StepError{Message: err.Error()}}
	}
	stepCtx := cloneMap(baseCtx)
	stepCtx["input"] = input

	h, ok := lookupHandler(step.Type)
	if !ok {
		return input, StepResult{OK: false, Error: &StepError{Message: fmt.Sprintf("unsupported step type %q", step.Type)}}
	}
	return h(ctx, Request{Step: step, Workflow: wf, Input: input, StepCtx: stepCtx})
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}
