package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func Defaults(paths Paths) Config {
	return Config{
		Daemon: DaemonConfig{
			BindAddress: "127.0.0.1",
			Port:        8765,
			StateDir:    paths.StateDir,
			ArtifactDir: paths.ArtifactDir,
			LogDir:      paths.LogDir,
		},
		Sources:   SourcesConfig{File: paths.SourcesFile},
		Workflows: WorkflowsConfig{Dir: paths.WorkflowsDir},
		Models:    ModelsConfig{},
	}
}

func Load(path string, paths Paths) (Config, error) {
	cfg := Defaults(paths)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config %s: %s", path, singleLine(err.Error()))
	}
	if cfg.Daemon.BindAddress == "" {
		cfg.Daemon.BindAddress = "127.0.0.1"
	}
	if cfg.Daemon.Port == 0 {
		cfg.Daemon.Port = 8765
	}
	if cfg.Daemon.StateDir == "" {
		cfg.Daemon.StateDir = paths.StateDir
	}
	if cfg.Daemon.ArtifactDir == "" {
		cfg.Daemon.ArtifactDir = paths.ArtifactDir
	}
	if cfg.Daemon.LogDir == "" {
		cfg.Daemon.LogDir = paths.LogDir
	}
	if cfg.Sources.File == "" {
		cfg.Sources.File = paths.SourcesFile
	}
	if cfg.Workflows.Dir == "" {
		cfg.Workflows.Dir = paths.WorkflowsDir
	}
	if cfg.Models == nil {
		cfg.Models = ModelsConfig{}
	}
	return cfg, nil
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func WriteInitial(paths Paths) error {
	if err := EnsureDirs(paths); err != nil {
		return err
	}
	cfg := Defaults(paths)
	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := writeIfMissing(paths.ConfigFile, configData, 0o644); err != nil {
		return err
	}
	sourcesYAML := []byte(`sources:
  - id: manual
    type: manual
    enabled: true
  # - id: notes
  #   type: filesystem
  #   enabled: true
  #   config:
  #     directory: ~/runloop-inbox
  #     glob: "*.md"
  # - id: heartbeat
  #   type: schedule
  #   enabled: true
  #   config:
  #     every: 1m
`)
	if err := writeIfMissing(paths.SourcesFile, sourcesYAML, 0o644); err != nil {
		return err
	}
	secretsYAML := []byte(`# Configure credential profiles once, then built-in steps can use them.
# secrets:
#   anthropic-api-key:
#     file: secrets/anthropic-api-key
#
# profiles:
#   claude:
#     env:
#       ANTHROPIC_API_KEY:
#         secret: anthropic-api-key
`)
	if err := writeIfMissing(paths.SecretsFile, secretsYAML, 0o600); err != nil {
		return err
	}
	workflow := []byte(`id: manual-hello
name: Manual Hello
enabled: true

triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    policy: once_per_item

steps:
  - id: echo
    type: transform
    input:
      message: "{{ inbox.normalized.message }}"
    output:
      result: "Hello from Runloop: {{ input.message }}"

sinks:
  - type: markdown
    path: report.md
`)
	if err := writeIfMissing(paths.WorkflowsDir+"/manual-hello.yaml", workflow, 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(paths.AuthToken); errors.Is(err, os.ErrNotExist) {
		token := make([]byte, 24)
		if _, err := rand.Read(token); err != nil {
			return err
		}
		return os.WriteFile(paths.AuthToken, []byte(hex.EncodeToString(token)+"\n"), 0o600)
	}
	return nil
}

func writeIfMissing(path string, data []byte, perm os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, perm)
}
