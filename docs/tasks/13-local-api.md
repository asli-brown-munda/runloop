# Local API

- [ ] **Health response detail** — Expand `GET /api/health` with version, uptime, DB status, and worker counts so operators can confirm the daemon is fully ready.
- [ ] **Wire missing endpoints** — Implement `POST /api/inbox/{id}/archive`, `POST /api/inbox/{id}/ignore`, `GET /api/runs/{id}`, `POST /api/runs/{id}/cancel`, `GET /api/sources`, and `POST /api/sources/{id}/test` per `docs/mvp.md` Phase 12.
- [ ] **Loopback-only enforcement** — Assert in `internal/web/server.go` that the listener is bound to `127.0.0.1` and reject any config that tries to bind elsewhere.
- [ ] **Auth token middleware** — Require the `~/.config/runloop/auth.token` value on every request and return 401 otherwise; rotate via `runloop init --rotate-token`.
