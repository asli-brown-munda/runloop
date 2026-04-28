# Daemon Supervisor

- [ ] **Graceful shutdown** — Handle SIGINT/SIGTERM in `internal/daemon/daemon.go`, cancel the root context, drain active runs, and close SQLite cleanly.
- [ ] **Worker supervisor** — Implement `internal/daemon/supervisor.go` so source sync, trigger evaluation, dispatch, and HTTP each run as named goroutines with restart on panic.
- [ ] **Lifecycle events** — Emit `daemon_events` on start, stop, migration, and worker restart from `internal/daemon/lifecycle.go`.
- [ ] **PID file / single-instance check** — Refuse to start if another daemon is already running against the same state directory.
