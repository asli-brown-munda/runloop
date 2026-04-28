package steps

import "testing"

func TestRenderMapUsesInboxAndInputContext(t *testing.T) {
	input := RenderMap(map[string]any{"message": "{{ inbox.normalized.message }}"}, map[string]any{
		"inbox": map[string]any{"normalized": map[string]any{"message": "hello"}},
	})
	out := RenderMap(map[string]any{"result": "Hello: {{ input.message }}"}, map[string]any{"input": input})

	if out["result"] != "Hello: hello" {
		t.Fatalf("result = %v", out["result"])
	}
}
