package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"runloop/internal/dispatch"
	"runloop/internal/inbox"
	"runloop/internal/runs"
	"runloop/internal/sources"
	"runloop/internal/sources/manual"
	"runloop/internal/store"
	"runloop/internal/triggers"
)

type API struct {
	store     *store.Store
	inbox     *inbox.Service
	evaluator *triggers.Evaluator
	engine    *runs.Engine
	sources   *sources.Manager
}

func (a *API) Routes(r chi.Router) {
	r.Get("/api/health", a.health)
	r.Get("/api/inbox", a.listInbox)
	r.Post("/api/inbox", a.addInbox)
	r.Get("/api/inbox/{id}", a.showInbox)
	r.Post("/api/inbox/{id}/archive", a.archiveInbox)
	r.Post("/api/inbox/{id}/ignore", a.ignoreInbox)
	r.Get("/api/workflows", a.listWorkflows)
	r.Get("/api/runs", a.listRuns)
	r.Get("/api/runs/{id}", a.showRun)
	r.Post("/api/runs/{id}/cancel", a.cancelRun)
	r.Get("/api/sources", a.listSources)
	r.Post("/api/sources/{id}/test", a.testSource)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) listInbox(w http.ResponseWriter, r *http.Request) {
	items, err := a.inbox.ListInboxItems(r.Context())
	writeResult(w, items, err)
}

type addInboxRequest struct {
	SourceID   string         `json:"source"`
	ExternalID string         `json:"externalId"`
	Title      string         `json:"title"`
	Payload    map[string]any `json:"payload"`
}

func (a *API) addInbox(w http.ResponseWriter, r *http.Request) {
	var req addInboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.SourceID == "" {
		req.SourceID = "manual"
	}
	item, version, changed, err := a.inbox.UpsertInboxItem(r.Context(), manual.Candidate(req.SourceID, req.ExternalID, req.Title, req.Payload))
	if err != nil {
		writeError(w, err)
		return
	}
	if changed {
		if err := a.evaluator.EvaluateInboxVersion(r.Context(), item, version); err != nil {
			writeError(w, err)
			return
		}
		if err := a.engine.Drain(r.Context()); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item, "version": version, "changed": changed})
}

func (a *API) showInbox(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := a.inbox.GetInboxItem(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := a.inbox.LatestInboxVersion(r.Context(), item.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	dispatches, err := a.store.ListDispatchesForItem(r.Context(), item.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	type dispatchWithRun struct {
		Dispatch dispatch.WorkflowDispatch `json:"dispatch"`
		Run      *runs.WorkflowRun         `json:"run,omitempty"`
	}
	drs := make([]dispatchWithRun, 0, len(dispatches))
	for _, d := range dispatches {
		run, ok, err := a.store.GetRunByDispatch(r.Context(), d.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		dwr := dispatchWithRun{Dispatch: d}
		if ok {
			dwr.Run = &run
		}
		drs = append(drs, dwr)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item":       item,
		"version":    version,
		"dispatches": drs,
	})
}

func (a *API) archiveInbox(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err == nil {
		err = a.inbox.ArchiveInboxItem(r.Context(), id)
	}
	writeResult(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) ignoreInbox(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err == nil {
		err = a.inbox.IgnoreInboxItem(r.Context(), id)
	}
	writeResult(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) listWorkflows(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListWorkflowDefinitions(r.Context())
	writeResult(w, items, err)
}

func (a *API) listRuns(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListRuns(r.Context())
	writeResult(w, items, err)
}

func (a *API) showRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	run, err := a.store.GetRun(r.Context(), id)
	writeResult(w, run, err)
}

func (a *API) cancelRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err == nil {
		err = a.store.UpdateRunStatus(r.Context(), id, runs.RunCancelled, "cancelled by user")
	}
	writeResult(w, map[string]any{"ok": err == nil}, err)
}

func (a *API) listSources(w http.ResponseWriter, r *http.Request) {
	type sourceInfo struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	out := []sourceInfo{}
	for _, source := range a.sources.List() {
		out = append(out, sourceInfo{ID: source.ID(), Type: source.Type()})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) testSource(w http.ResponseWriter, r *http.Request) {
	source, ok := a.sources.Get(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeResult(w, map[string]any{"ok": true}, source.Test(r.Context()))
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func writeResult[T any](w http.ResponseWriter, value T, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
