package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsUnknownConfigKeys(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		field string
	}{
		{name: "top level", input: "unknown: true\n", field: "unknown"},
		{name: "nested daemon", input: "daemon:\n  typo: true\n", field: "typo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			writeTestConfig(t, path, []byte(tc.input))

			_, err := Load(path, Paths{})
			if err == nil {
				t.Fatal("expected unknown key error")
			}
			if !strings.Contains(err.Error(), "field "+tc.field+" not found") {
				t.Fatalf("error = %q", err)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Fatalf("error should be one line, got %q", err)
			}
		})
	}
}

func TestLoadAllowsArbitraryModelKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeTestConfig(t, path, []byte(`models:
  claude:
    command: claude
    nested:
      arbitrary: true
`))

	cfg, err := Load(path, Paths{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Models["claude"]; !ok {
		t.Fatalf("models = %#v", cfg.Models)
	}
}

func writeTestConfig(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
