package secrets

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFileResolverResolveConnectionToken(t *testing.T) {
	dir := t.TempDir()
	writeSecretFile(t, dir, "github-work-token", "gh-test\n", 0o600)
	writeSecretsConfig(t, dir, `secrets:
  github-work-token:
    file: secrets/github-work-token
connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
`)

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveConnectionToken(context.Background(), "github.work")
	if err != nil {
		t.Fatal(err)
	}
	if value != "gh-test" {
		t.Fatalf("value = %q", value)
	}
	legacy, err := resolver.Resolve(context.Background(), "github-work-token")
	if err != nil {
		t.Fatal(err)
	}
	if legacy != "gh-test" {
		t.Fatalf("legacy value = %q", legacy)
	}
}

func TestFileResolverResolveConnectionEnv(t *testing.T) {
	dir := t.TempDir()
	writeSecretFile(t, dir, "claude-api-key", "sk-test\n", 0o600)
	writeSecretsConfig(t, dir, `secrets:
  claude-api-key:
    file: secrets/claude-api-key
profiles:
  claude:
    env:
      ANTHROPIC_API_KEY:
        secret: claude-api-key
connections:
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: claude-api-key
`)

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := resolver.ResolveConnectionEnv(context.Background(), "claude.default", "ANTHROPIC_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if value != "sk-test" {
		t.Fatalf("value = %q", value)
	}
	legacy, err := resolver.ResolveProfileEnv(context.Background(), "claude", "ANTHROPIC_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if legacy != "sk-test" {
		t.Fatalf("legacy value = %q", legacy)
	}
}

func TestFileResolverConnectionErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  string
		call func(*FileResolver) error
		want string
	}{
		{
			name: "invalid ref without dot",
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionToken(context.Background(), "github")
				return err
			},
			want: `connection ref "github" must be service.name`,
		},
		{
			name: "invalid ref with extra dot",
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionToken(context.Background(), "github.work.extra")
				return err
			},
			want: `connection ref "github.work.extra" must be service.name`,
		},
		{
			name: "missing connection",
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionToken(context.Background(), "github.work")
				return err
			},
			want: `connection "github.work" is not configured`,
		},
		{
			name: "missing static token secret",
			cfg: `connections:
  github:
    work:
      provider: static_token
`,
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionToken(context.Background(), "github.work")
				return err
			},
			want: `connection "github.work" tokenSecret is required`,
		},
		{
			name: "env connection used as token",
			cfg: `connections:
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: claude-api-key
`,
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionToken(context.Background(), "claude.default")
				return err
			},
			want: `connection "claude.default" provider "env" does not resolve tokens`,
		},
		{
			name: "missing env key",
			cfg: `connections:
  claude:
    default:
      provider: env
      env:
        OTHER:
          secret: other-secret
`,
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionEnv(context.Background(), "claude.default", "ANTHROPIC_API_KEY")
				return err
			},
			want: `connection "claude.default" does not configure env "ANTHROPIC_API_KEY"`,
		},
		{
			name: "missing env secret",
			cfg: `connections:
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY: {}
`,
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionEnv(context.Background(), "claude.default", "ANTHROPIC_API_KEY")
				return err
			},
			want: `connection "claude.default" env "ANTHROPIC_API_KEY" secret is required`,
		},
		{
			name: "static token connection used as env",
			cfg: `connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-token
`,
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionEnv(context.Background(), "github.work", "ANTHROPIC_API_KEY")
				return err
			},
			want: `connection "github.work" provider "static_token" does not resolve env "ANTHROPIC_API_KEY"`,
		},
		{
			name: "unsupported provider",
			cfg: `connections:
  github:
    work:
      provider: oauth
`,
			call: func(r *FileResolver) error {
				_, err := r.ResolveConnectionToken(context.Background(), "github.work")
				return err
			},
			want: `connection "github.work" provider "oauth" is unsupported`,
		},
		{
			name: "test env connection with no env",
			cfg: `connections:
  claude:
    default:
      provider: env
`,
			call: func(r *FileResolver) error {
				return r.TestConnection(context.Background(), "claude.default")
			},
			want: `connection "claude.default" env must configure at least one variable`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.cfg != "" {
				writeSecretsConfig(t, dir, tt.cfg)
			}
			resolver, err := NewFileResolver(dir)
			if err != nil {
				t.Fatal(err)
			}
			err = tt.call(resolver)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestFileResolverListConnections(t *testing.T) {
	dir := t.TempDir()
	writeSecretsConfig(t, dir, `connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
    personal:
      provider: static_token
      tokenSecret: github-personal-token
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: claude-api-key
`)

	resolver, err := NewFileResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := resolver.ListConnections()
	want := []Connection{
		{Service: "claude", Name: "default", Provider: "env"},
		{Service: "github", Name: "personal", Provider: "static_token"},
		{Service: "github", Name: "work", Provider: "static_token"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("connections = %#v, want %#v", got, want)
	}
}

func TestFileResolverTestConnection(t *testing.T) {
	t.Run("static token and env providers pass", func(t *testing.T) {
		dir := t.TempDir()
		writeSecretFile(t, dir, "github-work-token", "gh-test", 0o600)
		writeSecretFile(t, dir, "claude-api-key", "sk-test", 0o600)
		writeSecretsConfig(t, dir, `secrets:
  github-work-token:
    file: secrets/github-work-token
  claude-api-key:
    file: secrets/claude-api-key
connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: claude-api-key
`)
		resolver, err := NewFileResolver(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := resolver.TestConnection(context.Background(), "github.work"); err != nil {
			t.Fatalf("github.work test: %v", err)
		}
		if err := resolver.TestConnection(context.Background(), "claude.default"); err != nil {
			t.Fatalf("claude.default test: %v", err)
		}
	})

	t.Run("wraps unsafe secret permission errors", func(t *testing.T) {
		dir := t.TempDir()
		writeSecretFile(t, dir, "github-work-token", "gh-test", 0o644)
		writeSecretsConfig(t, dir, `secrets:
  github-work-token:
    file: secrets/github-work-token
connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
`)
		resolver, err := NewFileResolver(dir)
		if err != nil {
			t.Fatal(err)
		}
		err = resolver.TestConnection(context.Background(), "github.work")
		if err == nil {
			t.Fatal("expected error")
		}
		want := `connection "github.work" token: secret "github-work-token" file must not be group- or world-readable`
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), want)
		}
	})
}

func writeSecretFile(t *testing.T, dir, name, value string, perm os.FileMode) {
	t.Helper()
	secretsDir := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, name), []byte(value), perm); err != nil {
		t.Fatal(err)
	}
}

func writeSecretsConfig(t *testing.T, dir, yaml string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}
