package sources_test

import (
	"testing"

	"runloop/internal/config"
	"runloop/internal/sources"
	_ "runloop/internal/sources/filesystem"
	_ "runloop/internal/sources/manual"
	_ "runloop/internal/sources/schedule"
)

func TestLoadManagerSkipsDisabledAndSorts(t *testing.T) {
	tmp := t.TempDir()
	file := config.SourcesFile{
		Sources: []config.SourceEntry{
			{ID: "manual", Type: "manual", Enabled: true},
			{ID: "notes", Type: "filesystem", Enabled: true, Config: map[string]any{"directory": tmp}},
			{ID: "heartbeat", Type: "schedule", Enabled: true, Config: map[string]any{"every": "1m"}},
			{ID: "off", Type: "manual", Enabled: false},
		},
	}
	manager, err := sources.LoadManager(file)
	if err != nil {
		t.Fatalf("LoadManager: %v", err)
	}
	got := manager.List()
	if len(got) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(got))
	}
	wantOrder := []string{"heartbeat", "manual", "notes"}
	for i, source := range got {
		if source.ID() != wantOrder[i] {
			t.Fatalf("position %d: want %q, got %q", i, wantOrder[i], source.ID())
		}
	}
	if _, ok := manager.Get("off"); ok {
		t.Fatal("disabled source should not be registered")
	}
	if _, ok := manager.Get("manual"); !ok {
		t.Fatal("manual source missing")
	}
}

func TestLoadManagerRejectsUnknownType(t *testing.T) {
	file := config.SourcesFile{
		Sources: []config.SourceEntry{{ID: "x", Type: "bogus", Enabled: true}},
	}
	if _, err := sources.LoadManager(file); err == nil {
		t.Fatal("expected error for unknown source type")
	}
}

func TestLoadManagerRejectsDuplicateID(t *testing.T) {
	file := config.SourcesFile{
		Sources: []config.SourceEntry{
			{ID: "dup", Type: "manual", Enabled: true},
			{ID: "dup", Type: "manual", Enabled: true},
		},
	}
	if _, err := sources.LoadManager(file); err == nil {
		t.Fatal("expected duplicate ID error")
	}
}
