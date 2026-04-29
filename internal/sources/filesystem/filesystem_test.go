package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"runloop/internal/sources"
)

type fakeWatcher struct {
	events chan fsnotify.Event
	errors chan error
	added  chan string
	closed bool
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events: make(chan fsnotify.Event, 4),
		errors: make(chan error, 1),
		added:  make(chan string, 1),
	}
}

func (w *fakeWatcher) Add(name string) error {
	w.added <- name
	return nil
}

func (w *fakeWatcher) Close() error {
	w.closed = true
	return nil
}

func (w *fakeWatcher) Events() <-chan fsnotify.Event { return w.events }
func (w *fakeWatcher) Errors() <-chan error          { return w.errors }

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

func TestWaitForChangeReturnsForMatchingFileEvent(t *testing.T) {
	dir := t.TempDir()
	src := newSource(t, dir, map[string]any{"glob": "*.md"})
	watcher := newFakeWatcher()
	src.newWatcher = func() (fileWatcher, error) { return watcher, nil }

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		errc <- src.WaitForChange(ctx)
	}()

	select {
	case got := <-watcher.added:
		if got != dir {
			t.Fatalf("watcher added %q, want %q", got, dir)
		}
	case <-ctx.Done():
		t.Fatal("watcher was not registered")
	}

	watcher.events <- fsnotify.Event{Name: filepath.Join(dir, "skip.txt"), Op: fsnotify.Write}
	select {
	case err := <-errc:
		t.Fatalf("non-matching event returned early: %v", err)
	case <-time.After(10 * time.Millisecond):
	}

	watcher.events <- fsnotify.Event{Name: filepath.Join(dir, "keep.md"), Op: fsnotify.Write}
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("WaitForChange: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("matching event did not wake source")
	}
	if !watcher.closed {
		t.Fatal("watcher was not closed")
	}
}

func TestTestRejectsMissingDir(t *testing.T) {
	src := &Source{id: "notes", directory: "/this/should/not/exist", glob: "*", entityType: defaultEntityType}
	if err := src.Test(context.Background()); err == nil {
		t.Fatal("expected error for missing directory")
	}
}
