package workflows

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
	ID       string         `yaml:"id" json:"id"`
	Type     string         `yaml:"type" json:"type"`
	Input    map[string]any `yaml:"input" json:"input"`
	Output   map[string]any `yaml:"output" json:"output"`
	Command  string         `yaml:"command" json:"command"`
	Timeout  string         `yaml:"timeout" json:"timeout"`
	Duration string         `yaml:"duration" json:"duration"`
	Retry    RetryPolicy    `yaml:"retry" json:"retry"`
}

type RetryPolicy struct {
	MaxAttempts int    `yaml:"maxAttempts" json:"maxAttempts"`
	Backoff     string `yaml:"backoff" json:"backoff"`
	Delay       string `yaml:"delay" json:"delay"`
}

type Sink struct {
	Type string `yaml:"type" json:"type"`
	Path string `yaml:"path" json:"path"`
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
}
