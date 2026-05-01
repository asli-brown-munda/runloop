package steps

import (
	"context"
	"fmt"
	"strings"

	"runloop/internal/workflows"
)

func ResolveEnv(ctx context.Context, req Request, extra map[string]string) ([]string, error) {
	merged := envListToMap(req.BaseEnv)
	for name, value := range extra {
		merged[name] = value
	}
	for name, value := range req.Step.Env {
		resolved, err := resolveEnvValue(ctx, req, value)
		if err != nil {
			return nil, fmt.Errorf("env %s: %w", name, err)
		}
		merged[name] = resolved
	}
	return envMapToList(merged), nil
}

func resolveEnvValue(ctx context.Context, req Request, value workflows.EnvValue) (string, error) {
	switch value.Kind {
	case workflows.EnvLiteral:
		return value.Literal, nil
	case workflows.EnvSecret:
		if req.Secrets == nil {
			return "", fmt.Errorf("secret resolver is not configured")
		}
		return req.Secrets.Resolve(ctx, value.Secret)
	case workflows.EnvFromProfile:
		if req.Secrets == nil {
			return "", fmt.Errorf("secret resolver is not configured")
		}
		profile, name, ok := strings.Cut(value.From, ".")
		if !ok || profile == "" || name == "" {
			return "", fmt.Errorf("profile reference %q must be profile.ENV_NAME", value.From)
		}
		return req.Secrets.ResolveProfileEnv(ctx, profile, name)
	default:
		return "", fmt.Errorf("invalid env value")
	}
}

func ResolveWorkdir(step workflows.Step, stepCtx map[string]any) (string, error) {
	if step.Workdir != "" {
		rendered, err := RenderValue(step.Workdir, stepCtx)
		if err != nil {
			return "", err
		}
		dir, ok := rendered.(string)
		if !ok {
			return "", fmt.Errorf("workdir must render to string")
		}
		return dir, nil
	}
	if value, ok := lookup(stepCtx, "runloop.workspace"); ok {
		if dir, ok := value.(string); ok {
			return dir, nil
		}
	}
	return "", nil
}

func envListToMap(in []string) map[string]string {
	out := map[string]string{}
	for _, item := range in {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		out[name] = value
	}
	return out
}

func envMapToList(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for name, value := range in {
		out = append(out, name+"="+value)
	}
	return out
}
