package workflows

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Workflow struct {
	ID          string      `yaml:"id" json:"id"`
	Name        string      `yaml:"name" json:"name"`
	Enabled     bool        `yaml:"enabled" json:"enabled"`
	Permissions Permissions `yaml:"permissions" json:"permissions"`
	Triggers    []Trigger   `yaml:"triggers" json:"triggers"`
	Steps       []Step      `yaml:"steps" json:"steps"`
	Sinks       []Sink      `yaml:"sinks" json:"sinks"`
}

type Permissions struct {
	Shell bool `yaml:"shell" json:"shell"`
}

type Trigger struct {
	Type       string `yaml:"type" json:"type"`
	Source     string `yaml:"source" json:"source"`
	EntityType string `yaml:"entityType" json:"entityType"`
	Policy     string `yaml:"policy" json:"policy"`
}

type Step struct {
	ID             string              `yaml:"id" json:"id"`
	Type           string              `yaml:"type" json:"type"`
	Input          map[string]any      `yaml:"input" json:"input"`
	Output         map[string]any      `yaml:"output" json:"output"`
	Command        string              `yaml:"command" json:"command"`
	Timeout        string              `yaml:"timeout" json:"timeout"`
	Duration       string              `yaml:"duration" json:"duration"`
	Workdir        string              `yaml:"workdir" json:"workdir"`
	Env            map[string]EnvValue `yaml:"env" json:"env"`
	Prompt         string              `yaml:"prompt" json:"prompt"`
	Model          string              `yaml:"model" json:"model"`
	PermissionMode string              `yaml:"permissionMode" json:"permissionMode"`
	Auth           string              `yaml:"auth" json:"auth"`
	Connection     string              `yaml:"connection" json:"connection"`
	Args           []string            `yaml:"args" json:"args"`
	Retry          RetryPolicy         `yaml:"retry" json:"retry"`
}

type EnvValueKind string

const (
	EnvLiteral     EnvValueKind = "literal"
	EnvSecret      EnvValueKind = "secret"
	EnvFromProfile EnvValueKind = "from"
)

type EnvValue struct {
	Kind    EnvValueKind `json:"kind"`
	Literal string       `json:"literal,omitempty"`
	Secret  string       `json:"secret,omitempty"`
	From    string       `json:"from,omitempty"`
}

func (v *EnvValue) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		*v = EnvValue{Kind: EnvLiteral, Literal: value}
		return nil
	case yaml.MappingNode:
		if len(node.Content) != 2 {
			return fmt.Errorf("env value must contain exactly one of secret or from")
		}
		key := node.Content[0].Value
		var value string
		if err := node.Content[1].Decode(&value); err != nil {
			return err
		}
		switch key {
		case "secret":
			*v = EnvValue{Kind: EnvSecret, Secret: value}
		case "from":
			*v = EnvValue{Kind: EnvFromProfile, From: value}
		default:
			return fmt.Errorf("unknown env value field %q", key)
		}
		return nil
	default:
		return fmt.Errorf("env value must be a string or mapping")
	}
}

type RetryPolicy struct {
	MaxAttempts int    `yaml:"maxAttempts" json:"maxAttempts"`
	Backoff     string `yaml:"backoff" json:"backoff"`
	Delay       string `yaml:"delay" json:"delay"`
}

type Sink struct {
	Type string `yaml:"type" json:"type"`
	Path string `yaml:"path" json:"path"`
	Body string `yaml:"body" json:"body"`
}

type Definition struct {
	ID         int64  `json:"id"`
	WorkflowID string `json:"workflowID"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
}

type Version struct {
	ID           int64    `json:"id"`
	DefinitionID int64    `json:"definitionID"`
	Version      int      `json:"version"`
	Hash         string   `json:"hash"`
	Path         string   `json:"path"`
	Workflow     Workflow `json:"workflow"`
	YAML         string   `json:"-"`
}
