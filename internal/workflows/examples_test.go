package workflows_test

import (
	"path/filepath"
	"testing"

	"runloop/internal/sinks"
	"runloop/internal/steps"
	_ "runloop/internal/steps/claude"
	_ "runloop/internal/steps/gitcheckout"
	_ "runloop/internal/steps/shell"
	_ "runloop/internal/steps/transform"
	_ "runloop/internal/steps/wait"
	"runloop/internal/workflows"
)

func TestExampleWorkflowsParseAndValidate(t *testing.T) {
	workflows.StepTypeValidator = steps.IsRegistered
	workflows.SinkTypeValidator = sinks.IsRegistered
	for _, name := range []string{"manual-hello.yaml", "github-pr-claude.yaml"} {
		t.Run(name, func(t *testing.T) {
			wf, _, err := workflows.ParseFile(filepath.Join("..", "..", "examples", "workflows", name))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if err := workflows.Validate(wf); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}
