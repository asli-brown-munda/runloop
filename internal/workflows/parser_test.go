package workflows

import (
	"path/filepath"
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
