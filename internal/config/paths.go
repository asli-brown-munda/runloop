package config

import (
	"os"
	"path/filepath"
)

const AppName = "runloop"

type Paths struct {
	ConfigDir    string
	ConfigFile   string
	SourcesFile  string
	SecretsFile  string
	SecretsDir   string
	WorkflowsDir string
	StateDir     string
	DatabaseFile string
	ArtifactDir  string
	LogDir       string
	AuthToken    string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	configDir := filepath.Join(xdgDir("XDG_CONFIG_HOME", filepath.Join(home, ".config")), AppName)
	stateDir := filepath.Join(xdgDir("XDG_STATE_HOME", filepath.Join(home, ".local", "state")), AppName)
	shareDir := filepath.Join(xdgDir("XDG_DATA_HOME", filepath.Join(home, ".local", "share")), AppName)
	return Paths{
		ConfigDir:    configDir,
		ConfigFile:   filepath.Join(configDir, "config.yaml"),
		SourcesFile:  filepath.Join(configDir, "sources.yaml"),
		SecretsFile:  filepath.Join(configDir, "secrets.yaml"),
		SecretsDir:   filepath.Join(configDir, "secrets"),
		WorkflowsDir: filepath.Join(configDir, "workflows"),
		StateDir:     stateDir,
		DatabaseFile: filepath.Join(stateDir, "runloop.db"),
		ArtifactDir:  filepath.Join(shareDir, "artifacts"),
		LogDir:       filepath.Join(stateDir, "logs"),
		AuthToken:    filepath.Join(configDir, "auth.token"),
	}, nil
}

func ResolveRuntimePaths(paths Paths, cfg Config) Paths {
	resolved := paths
	if cfg.Daemon.StateDir != "" {
		resolved.StateDir = cfg.Daemon.StateDir
		resolved.DatabaseFile = filepath.Join(cfg.Daemon.StateDir, "runloop.db")
	}
	if cfg.Daemon.ArtifactDir != "" {
		resolved.ArtifactDir = cfg.Daemon.ArtifactDir
	}
	if cfg.Daemon.LogDir != "" {
		resolved.LogDir = cfg.Daemon.LogDir
	}
	return resolved
}

func EnsureDirs(paths Paths) error {
	for _, dir := range []string{paths.ConfigDir, paths.SecretsDir, paths.WorkflowsDir, paths.StateDir, paths.ArtifactDir, paths.LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func xdgDir(envName, fallback string) string {
	if value := os.Getenv(envName); value != "" {
		return value
	}
	return fallback
}
