package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"runloop/internal/secrets"
	"runloop/internal/workflows"
)

func TestReadinessWarnsForClaudeAutoWithoutProfile(t *testing.T) {
	diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
		Steps: []workflows.Step{{ID: "agent", Type: "claude", Auth: "auto"}},
	}, ReadinessOptions{
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
	})

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].Level != DiagnosticWarning {
		t.Fatalf("level = %q", diagnostics[0].Level)
	}
}

func TestReadinessErrorsForMissingGitCheckoutBinary(t *testing.T) {
	diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
		Steps: []workflows.Step{{ID: "checkout", Type: "git_checkout"}},
	}, ReadinessOptions{
		LookPath: func(name string) (string, error) {
			if name == "git" {
				return "", fmt.Errorf("missing")
			}
			return "/usr/bin/" + name, nil
		},
	})

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].Level != DiagnosticError || diagnostics[0].StepID != "checkout" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestReadinessErrorsForClaudeAPIKeyWithoutProfile(t *testing.T) {
	diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
		Steps: []workflows.Step{{ID: "agent", Type: "claude", Auth: "apiKey"}},
	}, ReadinessOptions{
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
	})

	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].Level != DiagnosticError {
		t.Fatalf("level = %q", diagnostics[0].Level)
	}
}

func TestReadinessErrorsForBrokenClaudeAutoProfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets", "anthropic-api-key"), []byte("sk-test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(`secrets:
  anthropic-api-key:
    file: secrets/anthropic-api-key
profiles:
  claude:
    env:
      ANTHROPIC_API_KEY:
        secret: anthropic-api-key
`), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver, err := secrets.NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}

	diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
		Steps: []workflows.Step{{ID: "agent", Type: "claude", Auth: "auto"}},
	}, ReadinessOptions{
		Secrets:  resolver,
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
	})

	if len(diagnostics) != 1 || diagnostics[0].Level != DiagnosticError {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
