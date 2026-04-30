package steps

import (
	"errors"
	"testing"
)

func TestRenderMapUsesInboxAndInputContext(t *testing.T) {
	input, err := RenderMap(map[string]any{"message": "{{ inbox.normalized.message }}"}, map[string]any{
		"inbox": map[string]any{"normalized": map[string]any{"message": "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderMap(map[string]any{"result": "Hello: {{ input.message }}"}, map[string]any{"input": input})
	if err != nil {
		t.Fatal(err)
	}

	if out["result"] != "Hello: hello" {
		t.Fatalf("result = %v", out["result"])
	}
}

func TestRenderMapReturnsTemplateErrorForMissingNestedPath(t *testing.T) {
	_, err := RenderMap(map[string]any{"message": "{{ inbox.normalized.message }}"}, map[string]any{
		"inbox": map[string]any{"normalized": map[string]any{}},
	})
	if err == nil {
		t.Fatal("expected missing template path error")
	}
	var templateErr *TemplateError
	if !errors.As(err, &templateErr) {
		t.Fatalf("expected TemplateError, got %T: %v", err, err)
	}
	if templateErr.Path != "inbox.normalized.message" {
		t.Fatalf("Path = %q", templateErr.Path)
	}
}
