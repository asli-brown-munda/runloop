# Dispatch And Run Engine

- [ ] **Run cancellation** — Implement `POST /api/runs/{id}/cancel` and `runloop runs cancel <id>`; the engine should observe the cancel signal between steps and mark the run `cancelled`.
- [ ] **Timed-out run status** — Add `timed_out` as a terminal status when a step exceeds its configured timeout, distinct from `failed`.
- [ ] **Run recovery on startup** — Replace the placeholder `internal/runs/recovery.go` with logic that marks orphaned `running` dispatches/runs as `failed` (or requeues them) when the daemon restarts.
- [ ] **Single-flight dispatch worker** — Document and enforce the MVP rule of one dispatch worker, sequential steps within a run, and parallel runs across workflows.
- [ ] **Per-run structured logs** — Write a per-run log file under `~/.local/state/runloop/logs/` in addition to step artifacts, so failures are inspectable without reading SQLite.
