# Secrets

- [ ] **Secret metadata table use** — Read and write `secret_metadata` for named secrets so workflows can reference them by ID.
- [ ] **Redactor pass on logs and artifacts** — Implement `internal/secrets/redactor.go` and apply it to step stdout/stderr and structured logs before persisting.
- [ ] **OS keychain backend stub** — Add an interface and a no-op backend in `internal/secrets/service.go` so a real keychain can be plugged in later.
- [ ] **File-backed secrets resolver** — Implement a `secrets.yaml` index in `~/.config/runloop/` that maps secret IDs to 0600-protected files under `~/.config/runloop/secrets/`. Resolve at sync and exec time so step env injection and source configs (e.g. `tokenSecret: <id>`) work without an OS keyring being unlocked at login. Verify file mode on read and refuse to load world- or group-readable files.
- [ ] **Hardened credential profile backend** — Move credential profiles behind the same resolver interface used by steps so workflows keep referring to capabilities like `claude` while storage can later move from file-backed secrets to an OS keychain or brokered credential store without workflow YAML changes.
- [ ] **Credential setup UX** — Add CLI/API support to initialize, validate, and repair credential profiles such as `profiles.claude` so users are not required to hand-edit `secrets.yaml` for common built-in steps.
- [ ] **Credential migration path** — When a hardened backend exists, migrate existing file-backed profile entries with an explicit local confirmation and leave workflow definitions unchanged.
