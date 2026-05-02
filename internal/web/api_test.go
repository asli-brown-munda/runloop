package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"runloop/internal/secrets"
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

type fakeConnectionInspector struct {
	connections []secrets.Connection
	testErr     error
}

func (f fakeConnectionInspector) ListConnections() []secrets.Connection {
	return f.connections
}

func (f fakeConnectionInspector) TestConnection(ctx context.Context, ref string) error {
	if f.testErr != nil {
		return f.testErr
	}
	for _, conn := range f.connections {
		if conn.Service+"."+conn.Name == ref {
			return nil
		}
	}
	return sql.ErrNoRows
}

func (f fakeConnectionInspector) ConnectionConfigured(ref string) bool {
	for _, conn := range f.connections {
		if conn.Service+"."+conn.Name == ref {
			return true
		}
	}
	return false
}

func testConnectionsAPI(t *testing.T, inspector fakeConnectionInspector) http.Handler {
	t.Helper()
	api := &API{connections: inspector}
	r := chi.NewRouter()
	api.Routes(r)
	return r
}

func TestConnectionsAPIList(t *testing.T) {
	handler := testConnectionsAPI(t, fakeConnectionInspector{connections: []secrets.Connection{
		{Service: "github", Name: "work", Provider: "static_token"},
	}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/connections", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", res.Code, res.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{
		"service":  "github",
		"name":     "work",
		"ref":      "github.work",
		"provider": "static_token",
	}}
	if len(out) != len(want) {
		t.Fatalf("unexpected connection count: %#v", out)
	}
	for key, value := range want[0] {
		if out[0][key] != value {
			t.Fatalf("connection[%q] = %#v, want %#v in %#v", key, out[0][key], value, out[0])
		}
	}
	for _, forbidden := range []string{"token", "tokenSecret", "secret", "secretID", "refreshToken", "path", "file"} {
		if _, ok := out[0][forbidden]; ok {
			t.Fatalf("connection response exposed %q: %#v", forbidden, out[0])
		}
	}
}

func TestConnectionsAPITestOK(t *testing.T) {
	handler := testConnectionsAPI(t, fakeConnectionInspector{connections: []secrets.Connection{
		{Service: "github", Name: "work", Provider: "static_token"},
	}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connections/github.work/test", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("test status=%d body=%s", res.Code, res.Body.String())
	}
	var out map[string]bool
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out["ok"] {
		t.Fatalf("expected ok response, got %#v", out)
	}
}

func TestConnectionsAPITestMissing(t *testing.T) {
	handler := testConnectionsAPI(t, fakeConnectionInspector{connections: []secrets.Connection{
		{Service: "github", Name: "work", Provider: "static_token"},
	}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connections/github.missing/test", nil)
	handler.ServeHTTP(res, req)
	if res.Code < 400 || res.Code >= 500 {
		t.Fatalf("expected client error, status=%d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "not found") && !strings.Contains(res.Body.String(), "not configured") {
		t.Fatalf("missing connection response is not helpful: %s", res.Body.String())
	}
}

func TestConnectionsAPITestInvalidRef(t *testing.T) {
	handler := testConnectionsAPI(t, fakeConnectionInspector{connections: []secrets.Connection{
		{Service: "github", Name: "work", Provider: "static_token"},
	}})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connections/github/test", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, status=%d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "service.name") {
		t.Fatalf("invalid ref response is not helpful: %s", res.Body.String())
	}
}

func TestConnectionsAPITestFailureIsSanitized(t *testing.T) {
	handler := testConnectionsAPI(t, fakeConnectionInspector{
		connections: []secrets.Connection{
			{Service: "github", Name: "work", Provider: "static_token"},
		},
		testErr: errors.New(`secret "gh-token" from /tmp/runloop-secrets.yaml could not be read`),
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connections/github.work/test", nil)
	handler.ServeHTTP(res, req)
	if res.Code < 400 {
		t.Fatalf("expected non-2xx, status=%d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `connection "github.work" test failed`) {
		t.Fatalf("sanitized failure response is not helpful: %s", body)
	}
	for _, leaked := range []string{"gh-token", "/tmp/runloop-secrets.yaml", "could not be read"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("failure response leaked %q: %s", leaked, body)
		}
	}
}

func TestConnectionsAPITestConfiguredSecretErrorIsSanitized(t *testing.T) {
	handler := testConnectionsAPI(t, fakeConnectionInspector{
		connections: []secrets.Connection{
			{Service: "github", Name: "work", Provider: "static_token"},
		},
		testErr: errors.New(`connection "github.work" token: secret "gh-token" is not configured`),
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/connections/github.work/test", nil)
	handler.ServeHTTP(res, req)
	if res.Code == http.StatusNotFound {
		t.Fatalf("configured broken connection returned not found: %s", res.Body.String())
	}
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected server error, status=%d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `connection "github.work" test failed`) {
		t.Fatalf("sanitized failure response is not helpful: %s", body)
	}
	for _, leaked := range []string{"gh-token", "token:", "secret", "not configured"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("failure response leaked %q: %s", leaked, body)
		}
	}
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
