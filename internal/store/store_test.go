package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"runloop/internal/artifacts"
	"runloop/internal/dispatch"
	"runloop/internal/runs"
	"runloop/internal/sources/manual"
	"runloop/internal/triggers"
)

func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}

func TestInboxUpsertDedupesAndVersionsChangedPayload(t *testing.T) {
	st, ctx := testStore(t)

	item1, version1, changed, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", "x", "X", map[string]any{"message": "hello"}))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || version1.Version != 1 {
		t.Fatalf("first upsert changed=%v version=%d", changed, version1.Version)
	}

	item2, version2, changed, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", "x", "X", map[string]any{"message": "hello"}))
	if err != nil {
		t.Fatal(err)
	}
	if item1.ID != item2.ID || changed || version2.ID != version1.ID {
		t.Fatalf("duplicate payload should reuse item/version: %#v %#v changed=%v", version1, version2, changed)
	}

	_, version3, changed, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", "x", "X", map[string]any{"message": "updated"}))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || version3.Version != 2 {
		t.Fatalf("changed payload should create version 2, got changed=%v version=%d", changed, version3.Version)
	}
}

func TestTriggerCreatesDispatchAndRunCompletesManualWorkflow(t *testing.T) {
	st, ctx := testStore(t)
	workflowPath := filepath.Join("..", "..", "examples", "workflows", "manual-hello.yaml")
	if _, _, err := st.LoadWorkflowFile(ctx, workflowPath); err != nil {
		t.Fatal(err)
	}
	item, version, _, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", "test-1", "First", map[string]any{"message": "hello"}))
	if err != nil {
		t.Fatal(err)
	}

	evaluator := triggers.NewEvaluator(st)
	if err := evaluator.EvaluateInboxVersion(ctx, item, version); err != nil {
		t.Fatal(err)
	}
	d, ok, err := st.ClaimQueuedDispatch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || d.Status != dispatch.StatusRunning {
		t.Fatalf("expected running dispatch, got %#v ok=%v", d, ok)
	}
	if err := st.UpdateDispatchStatus(ctx, d.ID, dispatch.StatusQueued); err != nil {
		t.Fatal(err)
	}

	engine := runs.NewEngine(st, artifacts.New(t.TempDir()))
	if err := engine.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	runList, err := st.ListRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(runList) != 1 || runList[0].Status != runs.RunCompleted {
		t.Fatalf("expected one completed run, got %#v", runList)
	}
	sink := filepath.Join(engineRootFromRun(t, st, ctx, runList[0].ID), "sinks", "report.md")
	if _, err := os.Stat(sink); err != nil {
		t.Fatalf("expected sink artifact %s: %v", sink, err)
	}
}

func engineRootFromRun(t *testing.T, st *Store, ctx context.Context, runID int64) string {
	t.Helper()
	var path string
	err := st.db.QueryRowContext(ctx, `SELECT path FROM sink_outputs WHERE workflow_run_id=? LIMIT 1`, runID).Scan(&path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(path))
}
