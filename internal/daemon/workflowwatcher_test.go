package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"runloop/internal/store"
)

func TestWorkflowWatcherReloadsChangedYAMLAndSkipsUnchanged(t *testing.T) {
	ctx := context.Background()
	st := openWorkflowWatcherStore(t, ctx)
	dir := t.TempDir()
	path := filepath.Join(dir, "manual-hello.yaml")
	if err := os.WriteFile(path, []byte(workflowWatcherYAML("first")), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := newWorkflowWatcher(dir, st, nil)

	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	latest := latestWorkflowVersion(t, ctx, st, "manual-hello")
	if latest != 1 {
		t.Fatalf("first load version=%d, want 1", latest)
	}

	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	latest = latestWorkflowVersion(t, ctx, st, "manual-hello")
	if latest != 1 {
		t.Fatalf("unchanged reload version=%d, want 1", latest)
	}

	if err := os.WriteFile(path, []byte(workflowWatcherYAML("second")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	latest = latestWorkflowVersion(t, ctx, st, "manual-hello")
	if latest != 2 {
		t.Fatalf("changed reload version=%d, want 2", latest)
	}
}

func TestWorkflowWatcherIgnoresInvalidYAMLAndKeepsReloading(t *testing.T) {
	ctx := context.Background()
	st := openWorkflowWatcherStore(t, ctx)
	dir := t.TempDir()
	path := filepath.Join(dir, "manual-hello.yaml")
	if err := os.WriteFile(path, []byte(workflowWatcherYAML("first")), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := newWorkflowWatcher(dir, st, nil)
	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("id: manual-hello\ntriggers:\n  - condition: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	latest := latestWorkflowVersion(t, ctx, st, "manual-hello")
	if latest != 1 {
		t.Fatalf("invalid reload version=%d, want 1", latest)
	}

	if err := os.WriteFile(path, []byte(workflowWatcherYAML("second")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	latest = latestWorkflowVersion(t, ctx, st, "manual-hello")
	if latest != 2 {
		t.Fatalf("valid reload after invalid version=%d, want 2", latest)
	}
}

func TestWorkflowWatcherIgnoresNonYAMLFiles(t *testing.T) {
	ctx := context.Background()
	st := openWorkflowWatcherStore(t, ctx)
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte(workflowWatcherYAML("first")), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := newWorkflowWatcher(dir, st, nil)

	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}
	defs, err := st.ListWorkflowDefinitions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 0 {
		t.Fatalf("non-YAML reload created workflow definitions: %#v", defs)
	}
}

func TestWorkflowWatcherRunReloadsChangedYAMLFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st := openWorkflowWatcherStore(t, ctx)
	dir := t.TempDir()
	path := filepath.Join(dir, "manual-hello.yaml")
	if err := os.WriteFile(path, []byte(workflowWatcherYAML("first")), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher := newWorkflowWatcher(dir, st, nil)
	watcher.debounce = 10 * time.Millisecond
	if err := watcher.reloadPath(ctx, path); err != nil {
		t.Fatal(err)
	}

	errc := make(chan error, 1)
	go func() {
		errc <- watcher.Run(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(path, []byte(workflowWatcherYAML("second")), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForWorkflowVersion(t, ctx, st, "manual-hello", 2)
	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("workflow watcher did not exit after context cancellation")
	}
}

func openWorkflowWatcherStore(t *testing.T, ctx context.Context) *store.Store {
	t.Helper()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "runloop.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func latestWorkflowVersion(t *testing.T, ctx context.Context, st *store.Store, workflowID string) int {
	t.Helper()
	version, err := st.LatestWorkflowVersionForDefinition(ctx, workflowID)
	if err != nil {
		t.Fatal(err)
	}
	return version.Version
}

func waitForWorkflowVersion(t *testing.T, ctx context.Context, st *store.Store, workflowID string, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if got := latestWorkflowVersion(t, ctx, st, workflowID); got >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for workflow %s version %d", workflowID, want)
		case <-tick.C:
		}
	}
}

func workflowWatcherYAML(result string) string {
	return `id: manual-hello
name: Manual Hello
enabled: true

triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    policy: once_per_item

steps:
  - id: echo
    type: transform
    output:
      result: "` + result + `"

sinks:
  - type: markdown
    path: report.md
`
}
