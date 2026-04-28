package steps

import (
	"fmt"
	"regexp"
	"strings"
)

var templatePattern = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

func RenderValue(value any, ctx map[string]any) any {
	switch v := value.(type) {
	case string:
		return renderString(v, ctx)
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = RenderValue(item, ctx)
		}
		return out
	default:
		return value
	}
}

func RenderMap(values map[string]any, ctx map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = RenderValue(value, ctx)
	}
	return out
}

func renderString(input string, ctx map[string]any) string {
	return templatePattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := templatePattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		value, ok := lookup(ctx, strings.TrimSpace(parts[1]))
		if !ok {
			return ""
		}
		return fmt.Sprint(value)
	})
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
