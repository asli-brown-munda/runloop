package steps

import (
	"context"
	"fmt"
	"os/exec"

	"runloop/internal/secrets"
	"runloop/internal/workflows"
)

type DiagnosticLevel string

const (
	DiagnosticWarning DiagnosticLevel = "warning"
	DiagnosticError   DiagnosticLevel = "error"
)

type Diagnostic struct {
	Level   DiagnosticLevel `json:"level"`
	StepID  string          `json:"stepID,omitempty"`
	Message string          `json:"message"`
}

type ReadinessOptions struct {
	Secrets  secrets.Resolver
	LookPath func(string) (string, error)
}

func CheckReadiness(ctx context.Context, wf workflows.Workflow, opts ReadinessOptions) []Diagnostic {
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	var diagnostics []Diagnostic
	for _, step := range wf.Steps {
		if step.Type == "git_checkout" {
			if _, err := lookPath("git"); err != nil {
				diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticError, StepID: step.ID, Message: "Git binary 'git' was not found in PATH"})
			}
			continue
		}
		if step.Type != "claude" {
			continue
		}
		if _, err := lookPath("claude"); err != nil {
			diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticError, StepID: step.ID, Message: "Claude CLI binary 'claude' was not found in PATH"})
			continue
		}
		auth := step.Auth
		if auth == "" {
			auth = "auto"
		}
		switch auth {
		case "apiKey":
			if opts.Secrets == nil {
				diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticError, StepID: step.ID, Message: "Claude API key auth requires profiles.claude env ANTHROPIC_API_KEY in secrets.yaml"})
				continue
			}
			if _, err := opts.Secrets.ResolveProfileEnv(ctx, "claude", "ANTHROPIC_API_KEY"); err != nil {
				diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticError, StepID: step.ID, Message: fmt.Sprintf("Claude API key auth is not ready: %v", err)})
			}
		case "auto":
			if opts.Secrets == nil {
				diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticWarning, StepID: step.ID, Message: "Claude API key profile is not configured; Runloop will rely on Claude CLI login state under HOME"})
				continue
			}
			inspector, ok := opts.Secrets.(secrets.ProfileInspector)
			if ok && !inspector.ProfileEnvConfigured("claude", "ANTHROPIC_API_KEY") {
				diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticWarning, StepID: step.ID, Message: "Claude API key profile is not configured; Runloop will rely on Claude CLI login state under HOME"})
				continue
			}
			if _, err := opts.Secrets.ResolveProfileEnv(ctx, "claude", "ANTHROPIC_API_KEY"); err != nil {
				level := DiagnosticWarning
				if ok {
					level = DiagnosticError
				}
				diagnostics = append(diagnostics, Diagnostic{Level: level, StepID: step.ID, Message: fmt.Sprintf("Claude API key profile is not ready: %v", err)})
			}
		case "login":
		default:
			diagnostics = append(diagnostics, Diagnostic{Level: DiagnosticError, StepID: step.ID, Message: fmt.Sprintf("Claude auth %q is unsupported", auth)})
		}
	}
	return diagnostics
}
