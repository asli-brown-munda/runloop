package workflows

import "fmt"

func Validate(wf Workflow) error {
	if wf.ID == "" {
		return fmt.Errorf("workflow id is required")
	}
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(wf.Triggers) == 0 {
		return fmt.Errorf("workflow requires at least one trigger")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow requires at least one step")
	}
	seen := map[string]bool{}
	for _, step := range wf.Steps {
		if step.ID == "" {
			return fmt.Errorf("step id is required")
		}
		if seen[step.ID] {
			return fmt.Errorf("duplicate step id %q", step.ID)
		}
		seen[step.ID] = true
		switch step.Type {
		case "transform", "shell", "wait":
		default:
			return fmt.Errorf("unsupported step type %q", step.Type)
		}
	}
	for _, sink := range wf.Sinks {
		switch sink.Type {
		case "markdown", "json", "file":
		default:
			return fmt.Errorf("unsupported sink type %q", sink.Type)
		}
	}
	return nil
}
