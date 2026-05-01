package steps

import (
	"context"
	"fmt"
	"os"

	"runloop/internal/secrets"
	"runloop/internal/workflows"
)

type Executor struct {
	secrets secrets.Resolver
	baseEnv func() []string
}

type Option func(*Executor)

func WithSecrets(resolver secrets.Resolver) Option {
	return func(e *Executor) { e.secrets = resolver }
}

func WithBaseEnv(fn func() []string) Option {
	return func(e *Executor) { e.baseEnv = fn }
}

func NewExecutor(opts ...Option) *Executor {
	e := &Executor{baseEnv: MinimalEnvFromOS}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

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
	return h(ctx, Request{Step: step, Workflow: wf, Input: input, StepCtx: stepCtx, Secrets: e.secrets, BaseEnv: e.baseEnv()})
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func MinimalEnvFromOS() []string {
	keys := []string{"PATH", "HOME", "USER", "TERM"}
	var env []string
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
