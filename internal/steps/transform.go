package steps

import (
	"fmt"
	"regexp"
	"strings"
)

var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

type TemplateError struct {
	Path string
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("missing template value for %q", e.Path)
}

func RenderValue(value any, ctx map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		return renderString(v, ctx)
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			rendered, err := RenderValue(item, ctx)
			if err != nil {
				return nil, err
			}
			out[key] = rendered
		}
		return out, nil
	default:
		return value, nil
	}
}

func RenderMap(values map[string]any, ctx map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for key, value := range values {
		rendered, err := RenderValue(value, ctx)
		if err != nil {
			return nil, err
		}
		out[key] = rendered
	}
	return out, nil
}

func renderString(input string, ctx map[string]any) (string, error) {
	var renderErr error
	out := templatePattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := templatePattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		path := strings.TrimSpace(parts[1])
		value, ok := lookup(ctx, path)
		if !ok {
			renderErr = &TemplateError{Path: path}
			return ""
		}
		return fmt.Sprint(value)
	})
	if renderErr != nil {
		return "", renderErr
	}
	return out, nil
}

func lookup(ctx map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = ctx
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
