package steps

import (
	"context"
	"sync"

	"runloop/internal/secrets"
	"runloop/internal/workflows"
)

// Request is the opaque context passed to a step handler.
// Adding fields here is non-breaking for existing handlers.
type Request struct {
	Step     workflows.Step
	Workflow workflows.Workflow
	Input    map[string]any // already-rendered step.Input
	StepCtx  map[string]any // baseCtx with "input" merged in (used for templating)
	Secrets  secrets.Resolver
	BaseEnv  []string
}

// Handler executes one step. The first return is the input echoed back
// (preserves the existing executor contract), the second is the result.
type Handler func(ctx context.Context, req Request) (map[string]any, StepResult)

var (
	registryMu sync.RWMutex
	registry   = map[string]Handler{}
)

// Register adds a handler for the given step type. Panics on empty type
// or duplicate registration (mirrors how Go's database/sql.Register behaves).
func Register(typ string, h Handler) {
	if typ == "" {
		panic("steps: Register called with empty type")
	}
	if h == nil {
		panic("steps: Register called with nil handler for type " + typ)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[typ]; dup {
		panic("steps: duplicate registration for type " + typ)
	}
	registry[typ] = h
}

func lookupHandler(typ string) (Handler, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	h, ok := registry[typ]
	return h, ok
}

// IsRegistered is used by the workflow validator to check unknown types.
func IsRegistered(typ string) bool {
	_, ok := lookupHandler(typ)
	return ok
}
