package workflows

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ParseFile(path string) (Workflow, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Workflow{}, nil, err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return Workflow{}, nil, err
	}
	if err := validateTriggerFields(&node); err != nil {
		return Workflow{}, nil, err
	}
	var wf Workflow
	if err := node.Decode(&wf); err != nil {
		return Workflow{}, nil, err
	}
	return wf, data, nil
}

func validateTriggerFields(node *yaml.Node) error {
	root := documentRoot(node)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Value != "triggers" {
			continue
		}
		return validateTriggerSequence(root.Content[i+1])
	}
	return nil
}

func documentRoot(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func validateTriggerSequence(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return nil
	}
	allowed := map[string]bool{
		"type":       true,
		"source":     true,
		"entityType": true,
		"policy":     true,
	}
	for _, trigger := range node.Content {
		if trigger.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(trigger.Content); i += 2 {
			key := trigger.Content[i]
			if !allowed[key.Value] {
				return fmt.Errorf("unknown trigger field %q at line %d", key.Value, key.Line)
			}
		}
	}
	return nil
}
