package workflows

import "context"

type Registry struct {
	loader interface {
		LoadWorkflowDir(context.Context, string) ([]Version, error)
	}
}

func NewRegistry(loader interface {
	LoadWorkflowDir(context.Context, string) ([]Version, error)
}) *Registry {
	return &Registry{loader: loader}
}

func (r *Registry) LoadDir(ctx context.Context, dir string) ([]Version, error) {
	return r.loader.LoadWorkflowDir(ctx, dir)
}
