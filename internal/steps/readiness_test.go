package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestReadinessClaudeConnectionValidHasNoDiagnostics(t *testing.T) {
	diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
		Steps: []workflows.Step{{ID: "agent", Type: "claude", Auth: "auto", Connection: "claude.default"}},
	}, ReadinessOptions{
		Secrets: readinessConnectionSecrets{
			connections: map[string]string{"claude.default.ANTHROPIC_API_KEY": "sk-connection"},
		},
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
	})

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestReadinessClaudeConnectionErrorsForBrokenOrMissingConnection(t *testing.T) {
	tests := []struct {
		name    string
		auth    string
		secrets secrets.Resolver
	}{
		{
			name: "auto broken",
			auth: "auto",
			secrets: readinessConnectionSecrets{
				connections: map[string]string{"claude.default.ANTHROPIC_API_KEY": "sk-connection"},
				errs:        map[string]error{"claude.default.ANTHROPIC_API_KEY": fmt.Errorf("secret file missing")},
			},
		},
		{
			name: "apiKey broken",
			auth: "apiKey",
			secrets: readinessConnectionSecrets{
				connections: map[string]string{"claude.default.ANTHROPIC_API_KEY": "sk-connection"},
				errs:        map[string]error{"claude.default.ANTHROPIC_API_KEY": fmt.Errorf("secret file missing")},
			},
		},
		{
			name:    "auto missing",
			auth:    "auto",
			secrets: readinessConnectionSecrets{},
		},
		{
			name:    "apiKey missing",
			auth:    "apiKey",
			secrets: readinessConnectionSecrets{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
				Steps: []workflows.Step{{ID: "agent", Type: "claude", Auth: tt.auth, Connection: "claude.default"}},
			}, ReadinessOptions{
				Secrets:  tt.secrets,
				LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
			})

			if len(diagnostics) != 1 || diagnostics[0].Level != DiagnosticError {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			if !strings.Contains(diagnostics[0].Message, "claude.default") || !strings.Contains(diagnostics[0].Message, "ANTHROPIC_API_KEY") {
				t.Fatalf("message = %q", diagnostics[0].Message)
			}
		})
	}
}

func TestReadinessClaudeLoginIgnoresBrokenConnection(t *testing.T) {
	diagnostics := CheckReadiness(context.Background(), workflows.Workflow{
		Steps: []workflows.Step{{ID: "agent", Type: "claude", Auth: "login", Connection: "claude.default"}},
	}, ReadinessOptions{
		Secrets: readinessConnectionSecrets{
			errs: map[string]error{"claude.default.ANTHROPIC_API_KEY": fmt.Errorf("broken connection")},
		},
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
	})

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
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

type readinessConnectionSecrets struct {
	connections map[string]string
	errs        map[string]error
}

func (s readinessConnectionSecrets) Resolve(ctx context.Context, id string) (string, error) {
	return "", fmt.Errorf("secret %q is not configured", id)
}

func (s readinessConnectionSecrets) ResolveProfileEnv(ctx context.Context, profile, name string) (string, error) {
	return "", fmt.Errorf("profile %q env %q is not configured", profile, name)
}

func (s readinessConnectionSecrets) ResolveConnectionEnv(ctx context.Context, ref string, name string) (string, error) {
	key := ref + "." + name
	if err := s.errs[key]; err != nil {
		return "", err
	}
	value, ok := s.connections[key]
	if !ok {
		return "", fmt.Errorf("connection %q does not configure env %q", ref, name)
	}
	return value, nil
}

func (s readinessConnectionSecrets) ConnectionConfigured(ref string) bool {
	_, ok := s.connections[ref+".ANTHROPIC_API_KEY"]
	return ok
}

func (s readinessConnectionSecrets) ListConnections() []secrets.Connection {
	return nil
}

func (s readinessConnectionSecrets) TestConnection(ctx context.Context, ref string) error {
	return nil
}
