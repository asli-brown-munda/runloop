package workflows

import (
	"os"

	"gopkg.in/yaml.v3"
)

func ParseFile(path string) (Workflow, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, nil, err
	}
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return Workflow{}, nil, err
	}
	return wf, data, nil
}
