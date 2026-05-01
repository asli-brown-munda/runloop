package sources

import (
	"context"
	"testing"

	"runloop/internal/config"
)

type optionProbeSecrets struct{}

func (optionProbeSecrets) Resolve(context.Context, string) (string, error) { return "", nil }
func (optionProbeSecrets) ResolveProfileEnv(context.Context, string, string) (string, error) {
	return "", nil
}

type optionProbeSource struct{ id string }

func (s optionProbeSource) ID() string   { return s.id }
func (s optionProbeSource) Type() string { return "option_probe" }
func (s optionProbeSource) Sync(context.Context, Cursor) ([]InboxCandidate, Cursor, error) {
	return nil, Cursor{}, nil
}
func (s optionProbeSource) Test(context.Context) error { return nil }

func TestLoadManagerPassesBuildOptionsToConstructors(t *testing.T) {
	const typ = "option_probe"
	seen := false
	Register(typ, func(id string, cfg map[string]any, opts BuildOptions) (Source, error) {
		seen = opts.Secrets != nil
		return optionProbeSource{id: id}, nil
	})

	manager, err := LoadManager(config.SourcesFile{Sources: []config.SourceEntry{
		{ID: "probe", Type: typ, Enabled: true},
	}}, BuildOptions{Secrets: optionProbeSecrets{}})
	if err != nil {
		t.Fatalf("LoadManager: %v", err)
	}
	if !seen {
		t.Fatal("constructor did not receive secrets resolver")
	}
	if _, ok := manager.Get("probe"); !ok {
		t.Fatal("probe source was not registered")
	}
}
