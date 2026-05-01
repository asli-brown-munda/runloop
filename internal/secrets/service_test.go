package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileResolverResolvesProfileEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}
	secretPath := filepath.Join(dir, "secrets", "anthropic-api-key")
	if err := os.WriteFile(secretPath, []byte("sk-test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := []byte(`secrets:
  anthropic-api-key:
    file: secrets/anthropic-api-key
profiles:
  claude:
    env:
      ANTHROPIC_API_KEY:
        secret: anthropic-api-key
`)
	if err := os.WriteFile(filepath.Join(dir, "secrets.yaml"), config, 0o644); err != nil {
		t.Fatal(err)
	}

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveProfileEnv(context.Background(), "claude", "ANTHROPIC_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if value != "sk-test" {
		t.Fatalf("value = %q", value)
	}
}

func TestFileResolverRejectsUnsafeSecretFilePermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}
	secretPath := filepath.Join(dir, "secrets", "token")
	if err := os.WriteFile(secretPath, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := []byte(`secrets:
  token:
    file: secrets/token
`)
	if err := os.WriteFile(filepath.Join(dir, "secrets.yaml"), config, 0o644); err != nil {
		t.Fatal(err)
	}

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "token"); err == nil {
		t.Fatal("expected unsafe permissions to fail")
	}
}
