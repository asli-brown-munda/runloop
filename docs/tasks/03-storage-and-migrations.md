# Storage And Migrations

- [ ] **Add forward-only migration runner test** — Verify migrations apply cleanly on a fresh DB and are no-ops on a populated DB in `internal/store/store_test.go`.
- [ ] **Implement repositories for missing MVP tables** — Add minimal repository methods for `trigger_evaluations`, `retry_attempts`, `artifacts`, `sink_outputs`, `daemon_events`, and `secret_metadata`. Only what the engine and API actually call.
- [ ] **Persist `daemon_events`** — Record startup, shutdown, migration, and worker lifecycle events so `runloop` can show recent daemon activity.
