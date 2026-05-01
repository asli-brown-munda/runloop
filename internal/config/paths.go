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
	configDir := filepath.Join(home, ".config", AppName)
	stateDir := filepath.Join(home, ".local", "state", AppName)
	shareDir := filepath.Join(home, ".local", "share", AppName)
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

func EnsureDirs(paths Paths) error {
	for _, dir := range []string{paths.ConfigDir, paths.SecretsDir, paths.WorkflowsDir, paths.StateDir, paths.ArtifactDir, paths.LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
