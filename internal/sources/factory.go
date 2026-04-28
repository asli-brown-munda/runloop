package sources

import (
	"fmt"
	"sync"

	"runloop/internal/config"
)

type Constructor func(id string, cfg map[string]any) (Source, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Constructor{}
)

func Register(typ string, ctor Constructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[typ] = ctor
}

func lookup(typ string) (Constructor, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	ctor, ok := registry[typ]
	return ctor, ok
}

func Build(entry config.SourceEntry) (Source, error) {
	if entry.ID == "" {
		return nil, fmt.Errorf("source entry missing id")
	}
	if entry.Type == "" {
		return nil, fmt.Errorf("source %q missing type", entry.ID)
	}
	ctor, ok := lookup(entry.Type)
	if !ok {
		return nil, fmt.Errorf("unknown source type %q for source %q", entry.Type, entry.ID)
	}
	return ctor(entry.ID, entry.Config)
}
