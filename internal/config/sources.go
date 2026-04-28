package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type SourcesFile struct {
	Sources []SourceEntry `yaml:"sources"`
}

type SourceEntry struct {
	ID      string         `yaml:"id"`
	Type    string         `yaml:"type"`
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
}

func LoadSourcesFile(path string) (SourcesFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return SourcesFile{}, nil
	}
	if err != nil {
		return SourcesFile{}, err
	}
	var file SourcesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return SourcesFile{}, err
	}
	return file, nil
}
