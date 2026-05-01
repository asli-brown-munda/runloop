package sources

import (
	"fmt"
	"sort"

	"runloop/internal/config"
)

type Manager struct {
	sources map[string]Source
}

func NewManager(items ...Source) *Manager {
	m := &Manager{sources: map[string]Source{}}
	for _, item := range items {
		m.sources[item.ID()] = item
	}
	return m
}

func LoadManager(file config.SourcesFile, opts ...BuildOptions) (*Manager, error) {
	m := &Manager{sources: map[string]Source{}}
	buildOpts := BuildOptions{}
	if len(opts) > 0 {
		buildOpts = opts[0]
	}
	for _, entry := range file.Sources {
		if !entry.Enabled {
			continue
		}
		if _, exists := m.sources[entry.ID]; exists {
			return nil, fmt.Errorf("duplicate source id %q", entry.ID)
		}
		source, err := Build(entry, buildOpts)
		if err != nil {
			return nil, err
		}
		m.sources[entry.ID] = source
	}
	return m, nil
}

func (m *Manager) List() []Source {
	out := make([]Source, 0, len(m.sources))
	for _, source := range m.sources {
		out = append(out, source)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

func (m *Manager) Get(id string) (Source, bool) {
	source, ok := m.sources[id]
	return source, ok
}

func (m *Manager) Register(source Source) error {
	if source == nil {
		return fmt.Errorf("nil source")
	}
	if _, exists := m.sources[source.ID()]; exists {
		return fmt.Errorf("duplicate source id %q", source.ID())
	}
	m.sources[source.ID()] = source
	return nil
}
