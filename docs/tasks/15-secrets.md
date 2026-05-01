# Secrets

- [ ] **Secret metadata table use** — Read and write `secret_metadata` for named secrets so workflows can reference them by ID.
- [ ] **Redactor pass on logs and artifacts** — Implement `internal/secrets/redactor.go` and apply it to step stdout/stderr and structured logs before persisting.
- [ ] **OS keychain backend stub** — Add an interface and a no-op backend in `internal/secrets/service.go` so a real keychain can be plugged in later.
- [ ] **File-backed secrets resolver** — Implement a `secrets.yaml` index in `~/.config/runloop/` that maps secret IDs to 0600-protected files under `~/.config/runloop/secrets/`. Resolve at sync and exec time so step env injection and source configs (e.g. `tokenSecret: <id>`) work without an OS keyring being unlocked at login. Verify file mode on read and refuse to load world- or group-readable files.
