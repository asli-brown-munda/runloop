package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathsUseXDGStyleRunloopDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}

	wantConfig := filepath.Join(home, ".config", "runloop", "config.yaml")
	if paths.ConfigFile != wantConfig {
		t.Fatalf("ConfigFile = %q, want %q", paths.ConfigFile, wantConfig)
	}
	if paths.DatabaseFile != filepath.Join(home, ".local", "state", "runloop", "runloop.db") {
		t.Fatalf("unexpected database path %q", paths.DatabaseFile)
	}
	if paths.ArtifactDir != filepath.Join(home, ".local", "share", "runloop", "artifacts") {
		t.Fatalf("unexpected artifact path %q", paths.ArtifactDir)
	}
	if paths.SecretsFile != filepath.Join(home, ".config", "runloop", "secrets.yaml") {
		t.Fatalf("unexpected secrets path %q", paths.SecretsFile)
	}
}

func TestWriteInitialCreatesConfigSamplesAndToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteInitial(paths); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{paths.ConfigFile, paths.SourcesFile, paths.SecretsFile, filepath.Join(paths.WorkflowsDir, "manual-hello.yaml"), paths.AuthToken} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}
