package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPathsUseXDGStyleRunloopDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

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

func TestDefaultPathsHonorXDGOverrides(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(t.TempDir(), "config-home")
	stateHome := filepath.Join(t.TempDir(), "state-home")
	dataHome := filepath.Join(t.TempDir(), "data-home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}

	if paths.ConfigFile != filepath.Join(configHome, "runloop", "config.yaml") {
		t.Fatalf("ConfigFile = %q", paths.ConfigFile)
	}
	if paths.DatabaseFile != filepath.Join(stateHome, "runloop", "runloop.db") {
		t.Fatalf("DatabaseFile = %q", paths.DatabaseFile)
	}
	if paths.ArtifactDir != filepath.Join(dataHome, "runloop", "artifacts") {
		t.Fatalf("ArtifactDir = %q", paths.ArtifactDir)
	}
}

func TestDefaultPathsDoNotUseTempDirUnlessExplicitlyConfigured(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "runloop-test-user")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}

	tempDir := filepath.Clean(os.TempDir())
	for name, path := range map[string]string{
		"ConfigDir":   paths.ConfigDir,
		"StateDir":    paths.StateDir,
		"ArtifactDir": paths.ArtifactDir,
		"LogDir":      paths.LogDir,
	} {
		clean := filepath.Clean(path)
		if clean == tempDir || strings.HasPrefix(clean, tempDir+string(filepath.Separator)) {
			t.Fatalf("%s = %q, should be derived from HOME when XDG variables are unset", name, path)
		}
	}
	if paths.ConfigDir != filepath.Join(home, ".config", "runloop") {
		t.Fatalf("ConfigDir = %q", paths.ConfigDir)
	}
}

func TestResolveRuntimePathsHonorsDaemonOverrides(t *testing.T) {
	base := Paths{
		ConfigDir:    filepath.Join("default", "config"),
		ConfigFile:   filepath.Join("default", "config", "config.yaml"),
		SourcesFile:  filepath.Join("default", "config", "sources.yaml"),
		SecretsFile:  filepath.Join("default", "config", "secrets.yaml"),
		SecretsDir:   filepath.Join("default", "config", "secrets"),
		WorkflowsDir: filepath.Join("default", "config", "workflows"),
		StateDir:     filepath.Join("default", "state"),
		DatabaseFile: filepath.Join("default", "state", "runloop.db"),
		ArtifactDir:  filepath.Join("default", "data", "artifacts"),
		LogDir:       filepath.Join("default", "state", "logs"),
		AuthToken:    filepath.Join("default", "config", "auth.token"),
	}
	cfg := Config{Daemon: DaemonConfig{
		StateDir:    filepath.Join("custom", "state"),
		ArtifactDir: filepath.Join("custom", "artifacts"),
		LogDir:      filepath.Join("custom", "logs"),
	}}

	paths := ResolveRuntimePaths(base, cfg)

	if paths.StateDir != filepath.Join("custom", "state") {
		t.Fatalf("StateDir = %q", paths.StateDir)
	}
	if paths.DatabaseFile != filepath.Join("custom", "state", "runloop.db") {
		t.Fatalf("DatabaseFile = %q", paths.DatabaseFile)
	}
	if paths.ArtifactDir != filepath.Join("custom", "artifacts") {
		t.Fatalf("ArtifactDir = %q", paths.ArtifactDir)
	}
	if paths.LogDir != filepath.Join("custom", "logs") {
		t.Fatalf("LogDir = %q", paths.LogDir)
	}
	if paths.ConfigFile != base.ConfigFile {
		t.Fatalf("ConfigFile changed to %q", paths.ConfigFile)
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

	sourcesData, err := os.ReadFile(paths.SourcesFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sourcesData), "connection: github.work") {
		t.Fatalf("sources sample should prefer connection: github.work:\n%s", sourcesData)
	}

	secretsData, err := os.ReadFile(paths.SecretsFile)
	if err != nil {
		t.Fatal(err)
	}
	secretsSample := string(secretsData)
	for _, want := range []string{
		"connections:",
		"claude:",
		"provider: env",
		"ANTHROPIC_API_KEY:",
		"Legacy profiles.claude",
	} {
		if !strings.Contains(string(secretsData), want) {
			t.Fatalf("secrets sample missing %q:\n%s", want, secretsData)
		}
	}
	connectionsIndex := strings.Index(secretsSample, "# connections:")
	secretsIndex := strings.Index(secretsSample, "# secrets:")
	profilesIndex := strings.Index(secretsSample, "# profiles:")
	if connectionsIndex == -1 || secretsIndex == -1 || profilesIndex == -1 {
		t.Fatalf("secrets sample should include connections, secrets, and profiles sections:\n%s", secretsData)
	}
	if !(connectionsIndex < secretsIndex && connectionsIndex < profilesIndex) {
		t.Fatalf("secrets sample should present connections before secrets and profiles:\n%s", secretsData)
	}
}
