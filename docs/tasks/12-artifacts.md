# Artifacts

- [ ] **Artifact metadata in SQLite** — Insert into `artifacts` for every inbox payload, step input/output, step log, and sink write, storing only the path and metadata.
- [ ] **Artifact GC policy (stub)** — Add a documented retention placeholder in `internal/artifacts/` so future cleanup has a clear hook.
