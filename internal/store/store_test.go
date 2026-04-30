package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"runloop/internal/artifacts"
	"runloop/internal/dispatch"
	"runloop/internal/runs"
	"runloop/internal/sources/manual"
	_ "runloop/internal/steps/shell"
	_ "runloop/internal/steps/transform"
	_ "runloop/internal/steps/wait"
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

func TestShellStepArtifactsArePersistedForRun(t *testing.T) {
	st, ctx := testStore(t)
	workflowPath := filepath.Join(t.TempDir(), "shell-artifacts.yaml")
	workflowYAML := `id: shell-artifacts
name: Shell Artifacts
enabled: true
permissions:
  shell: true
triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    policy: once_per_item
steps:
  - id: cmd
    type: shell
    command: "printf out; printf err >&2"
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.LoadWorkflowFile(ctx, workflowPath); err != nil {
		t.Fatal(err)
	}
	item, version, _, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", "shell-1", "Shell", map[string]any{"message": "hello"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := triggers.NewEvaluator(st).EvaluateInboxVersion(ctx, item, version); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	engine := runs.NewEngine(st, artifacts.New(root))
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
	stdoutPath := filepath.Join(root, "runs", fmt.Sprintf("run_%d", runList[0].ID), "steps", "cmd", "stdout.log")
	stderrPath := filepath.Join(root, "runs", fmt.Sprintf("run_%d", runList[0].ID), "steps", "cmd", "stderr.log")
	if got := readFile(t, stdoutPath); got != "out" {
		t.Fatalf("stdout artifact = %q", got)
	}
	if got := readFile(t, stderrPath); got != "err" {
		t.Fatalf("stderr artifact = %q", got)
	}

	var stepRunID int64
	if err := st.db.QueryRowContext(ctx, `SELECT id FROM step_runs WHERE workflow_run_id=? AND step_id='cmd'`, runList[0].ID).Scan(&stepRunID); err != nil {
		t.Fatal(err)
	}
	for typ, path := range map[string]string{"shell_stdout": stdoutPath, "shell_stderr": stderrPath} {
		var artifactID int64
		if err := st.db.QueryRowContext(ctx, `SELECT id FROM artifacts WHERE step_run_id=? AND type=? AND path=?`, stepRunID, typ, path).Scan(&artifactID); err != nil {
			t.Fatalf("expected artifact row for %s at %s: %v", typ, path, err)
		}
		output := readFile(t, filepath.Join(root, "runs", fmt.Sprintf("run_%d", runList[0].ID), "steps", "cmd", "output.json"))
		if !strings.Contains(output, fmt.Sprintf(`"id": %d`, artifactID)) || !strings.Contains(output, path) {
			t.Fatalf("output artifact does not include persisted artifact %d at %s: %s", artifactID, path, output)
		}
	}
}

func TestSourceCursorRoundTrip(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.EnsureSourceRow(ctx, "notes", "filesystem"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSourceCursor(ctx, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsZero() {
		t.Fatalf("expected zero cursor, got %#v", got)
	}
	if err := st.UpsertSourceCursor(ctx, "notes", "v1"); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetSourceCursor(ctx, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "v1" {
		t.Fatalf("want cursor v1, got %q", got.Value)
	}
	if err := st.UpsertSourceCursor(ctx, "notes", "v2"); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetSourceCursor(ctx, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "v2" {
		t.Fatalf("want cursor v2, got %q", got.Value)
	}
}

func TestWorkflowEnableDisableDoesNotCreateVersion(t *testing.T) {
	st, ctx := testStore(t)
	workflowPath := filepath.Join("..", "..", "examples", "workflows", "manual-hello.yaml")
	loaded, created, err := st.LoadWorkflowFile(ctx, workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first load to create workflow version")
	}

	disabled, err := st.SetWorkflowEnabled(ctx, "manual-hello", false)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled {
		t.Fatalf("expected workflow disabled, got %#v", disabled)
	}
	def, err := st.GetWorkflowDefinition(ctx, "manual-hello")
	if err != nil {
		t.Fatal(err)
	}
	if def.Enabled {
		t.Fatalf("expected stored workflow disabled, got %#v", def)
	}

	enabled, err := st.SetWorkflowEnabled(ctx, "manual-hello", true)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled.Enabled {
		t.Fatalf("expected workflow enabled, got %#v", enabled)
	}
	latest, err := st.LatestWorkflowVersionForDefinition(ctx, "manual-hello")
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != loaded.ID || latest.Version != loaded.Version {
		t.Fatalf("enable/disable should not create versions: loaded=%#v latest=%#v", loaded, latest)
	}
}

func TestLatestWorkflowVersionIncludesStoredYAML(t *testing.T) {
	st, ctx := testStore(t)
	workflowPath := filepath.Join("..", "..", "examples", "workflows", "manual-hello.yaml")
	if _, _, err := st.LoadWorkflowFile(ctx, workflowPath); err != nil {
		t.Fatal(err)
	}

	latest, err := st.LatestWorkflowVersionForDefinition(ctx, "manual-hello")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Workflow.ID != "manual-hello" {
		t.Fatalf("unexpected workflow: %#v", latest.Workflow)
	}
	if latest.YAML == "" {
		t.Fatal("expected latest version to include stored YAML")
	}
}

func TestListRecentDispatchesForWorkflowOrdersNewestFirstAndLimits(t *testing.T) {
	st, ctx := testStore(t)
	workflowPath := filepath.Join("..", "..", "examples", "workflows", "manual-hello.yaml")
	version, _, err := st.LoadWorkflowFile(ctx, workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	var created []int64
	for i := 0; i < 3; i++ {
		item, itemVersion, _, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", string(rune('a'+i)), "Item", map[string]any{"message": "hello"}))
		if err != nil {
			t.Fatal(err)
		}
		d, err := st.CreateDispatch(ctx, item.ID, itemVersion.ID, version.DefinitionID, version.ID)
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, d.ID)
	}

	dispatches, err := st.ListRecentDispatchesForWorkflow(ctx, version.DefinitionID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(dispatches) != 2 {
		t.Fatalf("expected 2 dispatches, got %#v", dispatches)
	}
	if dispatches[0].ID != created[2] || dispatches[1].ID != created[1] {
		t.Fatalf("expected newest dispatches first, got %#v", dispatches)
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
