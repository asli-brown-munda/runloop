# Sources

- [ ] **Source manager / registry** — Implement `internal/sources/manager.go` so multiple source instances can be registered, listed, and looked up by ID.
- [ ] **Filesystem source** — Implement `internal/sources/filesystem/filesystem.go` to watch a configured directory and emit normalized inbox candidates per file change.
- [ ] **Schedule source** — Implement `internal/sources/schedule/schedule.go` to emit synthetic inbox items on a cron-like schedule for time-based workflows.
- [ ] **`runloop sources list` and `sources test`** — Expose registered sources through the API and CLI, and run `Source.Test` from `POST /api/sources/{id}/test`.
- [ ] **Persist source cursors** — Save and restore source-specific cursors in `source_cursors` so polling sources resume correctly across daemon restarts.
