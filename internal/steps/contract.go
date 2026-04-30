package steps

type ArtifactRef struct {
	ID      int64  `json:"id,omitempty"`
	Path    string `json:"path"`
	Type    string `json:"type,omitempty"`
	Content string `json:"-"`
}

type StepError struct {
	Message string `json:"message"`
}

type StepResult struct {
	OK        bool           `json:"ok"`
	Data      map[string]any `json:"data,omitempty"`
	Artifacts []ArtifactRef  `json:"artifacts,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Error     *StepError     `json:"error,omitempty"`
}
