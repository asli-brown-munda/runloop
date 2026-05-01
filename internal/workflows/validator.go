package workflows

import "fmt"

// StepTypeValidator is set by the steps package (via the daemon) to delegate
// step-type validation to the steps registry. If nil, validation is skipped
// (useful for tests of the workflows package in isolation).
var StepTypeValidator func(string) bool

// SinkTypeValidator is set by the sinks package (via the daemon) to delegate
// sink-type validation to the sinks registry. If nil, validation is skipped
// (useful for tests of the workflows package in isolation).
var SinkTypeValidator func(string) bool

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
		if StepTypeValidator != nil && !StepTypeValidator(step.Type) {
			return fmt.Errorf("unsupported step type %q", step.Type)
		}
		for name, value := range step.Env {
			if name == "" {
				return fmt.Errorf("step %q env name is required", step.ID)
			}
			switch value.Kind {
			case EnvLiteral:
			case EnvSecret:
				if value.Secret == "" {
					return fmt.Errorf("step %q env %q secret is required", step.ID, name)
				}
			case EnvFromProfile:
				if value.From == "" {
					return fmt.Errorf("step %q env %q profile reference is required", step.ID, name)
				}
			default:
				return fmt.Errorf("step %q env %q has invalid value", step.ID, name)
			}
		}
	}
	for _, sink := range wf.Sinks {
		if SinkTypeValidator != nil && !SinkTypeValidator(sink.Type) {
			return fmt.Errorf("unsupported sink type %q", sink.Type)
		}
	}
	return nil
}
