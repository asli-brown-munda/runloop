package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"runloop/internal/sources"
)

func newSource(t *testing.T, dir string, cfg map[string]any) *Source {
	t.Helper()
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["directory"] = dir
	src, err := New("notes", cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return src
}

func TestSyncEmitsCandidatesAndAdvancesCursor(t *testing.T) {
	dir := t.TempDir()
	src := newSource(t, dir, nil)

	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates, cursor, err := src.Sync(context.Background(), sources.Cursor{})
	if err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("want 1 candidate, got %d", len(candidates))
	}
	got := candidates[0]
	if got.ExternalID != "a.md" || got.Title != "a.md" {
		t.Fatalf("unexpected candidate: %#v", got)
	}
	if got.RawPayload["content"] != "alpha" {
		t.Fatalf("want content alpha, got %v", got.RawPayload["content"])
	}
	if cursor.IsZero() {
		t.Fatalf("cursor should advance after a candidate")
	}

	candidates2, cursor2, err := src.Sync(context.Background(), cursor)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if len(candidates2) != 0 {
		t.Fatalf("want 0 candidates on no-change, got %d", len(candidates2))
	}
	if cursor2.Value != cursor.Value {
		t.Fatalf("cursor unexpectedly changed: %q -> %q", cursor.Value, cursor2.Value)
	}

	bumped := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(dir, "a.md"), bumped, bumped); err != nil {
		t.Fatal(err)
	}
	candidates3, _, err := src.Sync(context.Background(), cursor2)
	if err != nil {
		t.Fatalf("third Sync: %v", err)
	}
	if len(candidates3) != 1 {
		t.Fatalf("modified file should re-emit, got %d", len(candidates3))
	}
}

func TestSyncRespectsGlob(t *testing.T) {
	dir := t.TempDir()
	src := newSource(t, dir, map[string]any{"glob": "*.md"})

	if err := os.WriteFile(filepath.Join(dir, "keep.md"), []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates, _, err := src.Sync(context.Background(), sources.Cursor{})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ExternalID != "keep.md" {
		t.Fatalf("glob not applied: %#v", candidates)
	}
}

func TestTestRejectsMissingDir(t *testing.T) {
	src := &Source{id: "notes", directory: "/this/should/not/exist", glob: "*", entityType: defaultEntityType}
	if err := src.Test(context.Background()); err == nil {
		t.Fatal("expected error for missing directory")
	}
}
