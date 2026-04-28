package sources

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

func (m *Manager) List() []Source {
	out := make([]Source, 0, len(m.sources))
	for _, source := range m.sources {
		out = append(out, source)
	}
	return out
}

func (m *Manager) Get(id string) (Source, bool) {
	source, ok := m.sources[id]
	return source, ok
}
