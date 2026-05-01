# Sources

- [x] **Source manager / registry** — Implement `internal/sources/manager.go` so multiple source instances can be registered, listed, and looked up by ID.
- [x] **Filesystem source** — Implement `internal/sources/filesystem/filesystem.go` to watch a configured directory with `fsnotify` and emit normalized inbox candidates per file change.
- [x] **Schedule source** — Implement `internal/sources/schedule/schedule.go` to emit synthetic inbox items on a cron-like schedule for time-based workflows.
- [x] **`runloop sources list` and `sources test`** — Expose registered sources through the API and CLI, and run `Source.Test` from `POST /api/sources/{id}/test`.
- [x] **Persist source cursors** — Save and restore source-specific cursors in `source_cursors` so configured sources resume correctly across daemon restarts.
- [ ] **GitHub PR source** — Implement `internal/sources/github/github.go` that polls a configured search query (for example `is:pr review-requested:@me`) on an `every` cadence, emits one inbox candidate per PR, and includes the unresolved review-comment IDs in the normalized payload so a new comment creates a new inbox version. Resolve the GitHub token through a named secret (see `15-secrets.md`) rather than reading from the daemon environment.
