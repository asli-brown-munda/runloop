package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"runloop/internal/artifacts"
	"runloop/internal/inbox"
	"runloop/internal/runs"
	"runloop/internal/sources"
	"runloop/internal/sources/manual"
	"runloop/internal/steps"
	_ "runloop/internal/steps/transform"
	"runloop/internal/store"
	"runloop/internal/triggers"
)

func testWorkflowAPI(t *testing.T) (*store.Store, http.Handler) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, _, err := st.LoadWorkflowFile(ctx, filepath.Join("..", "..", "examples", "workflows", "manual-hello.yaml")); err != nil {
		t.Fatal(err)
	}
	inboxSvc := inbox.NewService(st)
	evaluator := triggers.NewEvaluator(st)
	engine := runs.NewEngine(st, artifacts.New(t.TempDir()))
	api := &API{
		store:     st,
		inbox:     inboxSvc,
		evaluator: evaluator,
		engine:    engine,
		sources:   sources.NewManager(manual.New("manual")),
		readiness: steps.ReadinessOptions{LookPath: func(string) (string, error) { return "/usr/bin/claude", nil }},
	}
	r := chi.NewRouter()
	api.Routes(r)
	return st, r
}

func TestWorkflowShowEndpointIncludesReadinessDiagnostics(t *testing.T) {
	st, handler := testWorkflowAPI(t)
	path := filepath.Join(t.TempDir(), "claude.yaml")
	data := []byte(`id: claude-demo
name: Claude Demo
enabled: true
triggers:
  - type: inbox
steps:
  - id: agent
    type: claude
    auth: auto
    prompt: "hello"
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.LoadWorkflowFile(context.Background(), path); err != nil {
		t.Fatal(err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workflows/claude-demo", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("show status=%d body=%s", res.Code, res.Body.String())
	}
	var out struct {
		Readiness []steps.Diagnostic `json:"readiness"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Readiness) != 1 || out.Readiness[0].Level != steps.DiagnosticWarning {
		t.Fatalf("readiness = %#v", out.Readiness)
	}
}

func TestRunShowEndpointIncludesSinkOutputs(t *testing.T) {
	_, handler := testWorkflowAPI(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/inbox", strings.NewReader(`{
		"source": "manual",
		"externalId": "sink-api",
		"title": "Sink API",
		"payload": {"message": "hello"}
	}`))
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("add inbox status=%d body=%s", res.Code, res.Body.String())
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("list runs status=%d body=%s", res.Code, res.Body.String())
	}
	var runList []runs.WorkflowRun
	if err := json.Unmarshal(res.Body.Bytes(), &runList); err != nil {
		t.Fatal(err)
	}
	if len(runList) != 1 {
		t.Fatalf("expected one run, got %#v", runList)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/runs/"+strconv.FormatInt(runList[0].ID, 10), nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("show run status=%d body=%s", res.Code, res.Body.String())
	}
	var out struct {
		Run         runs.WorkflowRun   `json:"run"`
		SinkOutputs []store.SinkOutput `json:"sinkOutputs"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Run.ID != runList[0].ID {
		t.Fatalf("expected run %d, got %#v", runList[0].ID, out.Run)
	}
	if len(out.SinkOutputs) != 1 {
		t.Fatalf("expected one sink output, got %#v", out.SinkOutputs)
	}
	if out.SinkOutputs[0].Type != "markdown" || !strings.Contains(out.SinkOutputs[0].Path, filepath.Join("sinks", "report.md")) {
		t.Fatalf("unexpected sink output: %#v", out.SinkOutputs[0])
	}
}

func TestWorkflowEnableDisableEndpoints(t *testing.T) {
	_, handler := testWorkflowAPI(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/manual-hello/disable", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", res.Code, res.Body.String())
	}
	var disabled struct {
		WorkflowID string `json:"workflowID"`
		Enabled    bool   `json:"enabled"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &disabled); err != nil {
		t.Fatal(err)
	}
	if disabled.WorkflowID != "manual-hello" || disabled.Enabled {
		t.Fatalf("unexpected disable response: %#v", disabled)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/workflows/manual-hello/enable", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", res.Code, res.Body.String())
	}
	var enabled struct {
		WorkflowID string `json:"workflowID"`
		Enabled    bool   `json:"enabled"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &enabled); err != nil {
		t.Fatal(err)
	}
	if enabled.WorkflowID != "manual-hello" || !enabled.Enabled {
		t.Fatalf("unexpected enable response: %#v", enabled)
	}
}

func TestWorkflowShowEndpointIncludesLatestYAMLAndRecentDispatches(t *testing.T) {
	st, handler := testWorkflowAPI(t)
	ctx := context.Background()
	version, err := st.LatestWorkflowVersionForDefinition(ctx, "manual-hello")
	if err != nil {
		t.Fatal(err)
	}
	item, itemVersion, _, err := st.UpsertInboxItem(ctx, manual.Candidate("manual", "show", "Show", map[string]any{"message": "hello"}))
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := st.CreateDispatch(ctx, item.ID, itemVersion.ID, version.DefinitionID, version.ID)
	if err != nil {
		t.Fatal(err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workflows/manual-hello", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("show status=%d body=%s", res.Code, res.Body.String())
	}
	var out struct {
		Definition struct {
			WorkflowID string `json:"workflowID"`
			Enabled    bool   `json:"enabled"`
		} `json:"definition"`
		YAML       string `json:"yaml"`
		Dispatches []struct {
			ID int64 `json:"ID"`
		} `json:"dispatches"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Definition.WorkflowID != "manual-hello" || !out.Definition.Enabled {
		t.Fatalf("unexpected definition: %#v", out.Definition)
	}
	if out.YAML == "" {
		t.Fatal("expected stored workflow YAML")
	}
	if len(out.Dispatches) != 1 || out.Dispatches[0].ID != dispatch.ID {
		t.Fatalf("unexpected dispatches: %#v", out.Dispatches)
	}
}

func TestWorkflowEndpointsReturnNotFoundForUnknownWorkflow(t *testing.T) {
	_, handler := testWorkflowAPI(t)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/workflows/missing"},
		{method: http.MethodPost, path: "/api/workflows/missing/enable"},
		{method: http.MethodPost, path: "/api/workflows/missing/disable"},
	} {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d body=%s", tc.method, tc.path, res.Code, res.Body.String())
		}
	}
}
