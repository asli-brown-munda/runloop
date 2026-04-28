package config

type Config struct {
	Daemon    DaemonConfig    `yaml:"daemon"`
	Sources   SourcesConfig   `yaml:"sources"`
	Workflows WorkflowsConfig `yaml:"workflows"`
	Models    ModelsConfig    `yaml:"models"`
}

type DaemonConfig struct {
	BindAddress string `yaml:"bindAddress"`
	Port        int    `yaml:"port"`
	StateDir    string `yaml:"stateDir"`
	ArtifactDir string `yaml:"artifactDir"`
	LogDir      string `yaml:"logDir"`
}

type SourcesConfig struct {
	File string `yaml:"file"`
}

type WorkflowsConfig struct {
	Dir string `yaml:"dir"`
}

type ModelsConfig map[string]any
