# Secrets

- [ ] **Secret metadata table use** — Read and write `secret_metadata` for named secrets so workflows can reference them by ID.
- [ ] **Redactor pass on logs and artifacts** — Implement `internal/secrets/redactor.go` and apply it to step stdout/stderr and structured logs before persisting.
- [ ] **OS keychain backend stub** — Add an interface and a no-op backend in `internal/secrets/service.go` so a real keychain can be plugged in later.
- [x] **File-backed secrets resolver** — Implemented as the storage layer behind `secrets.yaml` and `~/.config/runloop/secrets/`. Secret files must stay under the Runloop config directory, use restrictive file modes, and can back direct step env entries, legacy `tokenSecret`, legacy profiles, and the preferred `connections` model.
- [x] **Connections credential model** — Supersedes the earlier profile-first setup. New GitHub PR sources should reference `connection: github.work`; Claude API-key workflows should reference `connection: claude.default`. Implemented providers include `static_token`, `env`, and `github_user_device`.
- [ ] **Hardened connection backend** — Move connection storage behind a backend that can later use an OS keychain or brokered credential store while sources and workflows keep referring to stable connection IDs.
- [ ] **Credential setup UX** — Extend CLI/API support to initialize, validate, repair, and reconnect connections so users are not required to hand-edit `secrets.yaml` for common built-in integrations. Keep `profiles.claude` support as compatibility, not the primary UX.
- [ ] **Credential migration path** — When a hardened backend exists, migrate existing file-backed connection and legacy profile entries with an explicit local confirmation and leave workflow definitions unchanged.
