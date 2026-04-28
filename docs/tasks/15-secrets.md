# Secrets

- [ ] **Secret metadata table use** — Read and write `secret_metadata` for named secrets so workflows can reference them by ID.
- [ ] **Redactor pass on logs and artifacts** — Implement `internal/secrets/redactor.go` and apply it to step stdout/stderr and structured logs before persisting.
- [ ] **OS keychain backend stub** — Add an interface and a no-op backend in `internal/secrets/service.go` so a real keychain can be plugged in later.
