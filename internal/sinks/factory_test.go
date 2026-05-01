package sinks

import (
	"path/filepath"
	"strings"
	"testing"

	"runloop/internal/workflows"
)

func TestBuiltInSinksRenderOutputs(t *testing.T) {
	root := t.TempDir()
	data := map[string]any{"result": "ok"}

	jsonOutput, err := Render(Request{
		Root:  root,
		RunID: 42,
		Sink:  workflows.Sink{Type: "json", Path: "ignored.json"},
		Data:  data,
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonOutput.Path != filepath.Join(root, "runs", "run_42", "sinks", "report.json") {
		t.Fatalf("json path = %q", jsonOutput.Path)
	}
	if !strings.Contains(string(jsonOutput.Body), `"result": "ok"`) {
		t.Fatalf("json body = %s", jsonOutput.Body)
	}

	fileOutput, err := Render(Request{
		Root:  root,
		RunID: 42,
		Sink:  workflows.Sink{Type: "file", Path: "reports/result.txt", Body: "Result: {{ .result }}"},
		Data:  data,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fileOutput.Path != filepath.Join(root, "runs", "run_42", "sinks", "reports", "result.txt") {
		t.Fatalf("file path = %q", fileOutput.Path)
	}
	if string(fileOutput.Body) != "Result: ok" {
		t.Fatalf("file body = %q", fileOutput.Body)
	}
}

func TestRenderRejectsUnknownAndUnsafeSinks(t *testing.T) {
	root := t.TempDir()

	if _, err := Render(Request{Root: root, RunID: 1, Sink: workflows.Sink{Type: "email"}}); err == nil {
		t.Fatal("expected unknown sink type error")
	}

	for _, unsafePath := range []string{"/tmp/report.md", "../report.md", "nested/../../report.md"} {
		_, err := Render(Request{Root: root, RunID: 1, Sink: workflows.Sink{Type: "file", Path: unsafePath}})
		if err == nil {
			t.Fatalf("expected %q to be rejected", unsafePath)
		}
	}
}

func TestRegisterAddsCustomSinkHandler(t *testing.T) {
	Register("test-custom-render", func(req Request) (Output, error) {
		return Output{Path: filepath.Join(req.Root, "custom.txt"), Body: req.Sink.Body}, nil
	})

	output, err := Render(Request{
		Root: t.TempDir(),
		Sink: workflows.Sink{Type: "test-custom-render", Body: "custom body"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(output.Path) != "custom.txt" || output.Body != "custom body" {
		t.Fatalf("unexpected custom output: %#v", output)
	}
}

func TestRegisterRejectsInvalidHandlers(t *testing.T) {
	if !IsRegistered("json") {
		t.Fatal("expected json sink to be registered")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate registration to panic")
		}
	}()
	Register("json", func(Request) (Output, error) { return Output{}, nil })
}
