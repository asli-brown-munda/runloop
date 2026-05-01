package workflows

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStepEnvParsesLiteralSecretAndProfileReferences(t *testing.T) {
	data := []byte(`id: env-demo
name: Env Demo
enabled: true
triggers:
  - type: inbox
steps:
  - id: cmd
    type: shell
    env:
      LITERAL: value
      TOKEN:
        secret: api-token
      PROFILED:
        from: claude.ANTHROPIC_API_KEY
`)
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatal(err)
	}

	env := wf.Steps[0].Env
	if env["LITERAL"].Literal != "value" || env["LITERAL"].Kind != EnvLiteral {
		t.Fatalf("literal env = %#v", env["LITERAL"])
	}
	if env["TOKEN"].Secret != "api-token" || env["TOKEN"].Kind != EnvSecret {
		t.Fatalf("secret env = %#v", env["TOKEN"])
	}
	if env["PROFILED"].From != "claude.ANTHROPIC_API_KEY" || env["PROFILED"].Kind != EnvFromProfile {
		t.Fatalf("profile env = %#v", env["PROFILED"])
	}
}

func TestValidateRejectsInvalidEnvReferences(t *testing.T) {
	err := Validate(Workflow{
		ID:       "x",
		Name:     "X",
		Triggers: []Trigger{{Type: "inbox"}},
		Steps: []Step{{
			ID:   "cmd",
			Type: "shell",
			Env:  map[string]EnvValue{"TOKEN": {Kind: EnvSecret}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "TOKEN") || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected env validation error, got %v", err)
	}
}
