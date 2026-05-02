package workflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFileAcceptsManualHelloExample(t *testing.T) {
	wf, _, err := ParseFile(filepath.Join("..", "..", "examples", "workflows", "manual-hello.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if wf.ID != "manual-hello" || wf.Steps[0].Type != "transform" {
		t.Fatalf("unexpected workflow: %#v", wf)
	}
	if err := Validate(wf); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowStepConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	data := []byte(`id: claude-connection
name: Claude Connection
enabled: true
permissions:
  shell: true
triggers:
  - type: inbox
steps:
  - id: agent
    type: claude
    auth: auto
    connection: claude.default
    prompt: hello
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	wf, _, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := wf.Steps[0].Connection; got != "claude.default" {
		t.Fatalf("connection = %q, want %q", got, "claude.default")
	}
}

func TestValidateRejectsDuplicateStepIDs(t *testing.T) {
	err := Validate(Workflow{
		ID:       "x",
		Name:     "X",
		Triggers: []Trigger{{Type: "inbox"}},
		Steps: []Step{
			{ID: "same", Type: "transform"},
			{ID: "same", Type: "wait"},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate step id error")
	}
}

func TestValidateRejectsUnsupportedStepType(t *testing.T) {
	err := Validate(Workflow{
		ID:       "x",
		Name:     "X",
		Triggers: []Trigger{{Type: "inbox"}},
		Steps:    []Step{{ID: "bad", Type: "llm"}},
	})
	if err == nil {
		t.Fatal("expected unsupported step type error")
	}
}

func TestValidateRejectsUnsupportedSinkType(t *testing.T) {
	err := Validate(Workflow{
		ID:       "x",
		Name:     "X",
		Triggers: []Trigger{{Type: "inbox"}},
		Steps:    []Step{{ID: "ok", Type: "transform"}},
		Sinks:    []Sink{{Type: "email", Path: "report.md"}},
	})
	if err == nil {
		t.Fatal("expected unsupported sink type error")
	}
}

func TestParseFileRejectsUnknownTriggerFieldWithLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	data := []byte(`id: x
name: X
enabled: true
triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    condition: inbox.title != ""
steps:
  - id: ok
    type: transform
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected unknown trigger field error")
	}
	if !strings.Contains(err.Error(), `unknown trigger field "condition" at line 8`) {
		t.Fatalf("expected field and line in error, got %q", err.Error())
	}
}
