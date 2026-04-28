# Config And Paths

- [ ] **Validate config on load** — Reject unknown keys and surface a single readable error from `internal/config/loader.go` instead of silently ignoring fields.
- [ ] **Round-trip test for default paths** — Extend `internal/config/paths_test.go` to cover `XDG_*` overrides and the fallback to `~/.config`, `~/.local/state`, and `~/.local/share`.
- [ ] **Honor explicit `daemon.stateDir` / `artifactDir` / `logDir`** — When set in `config.yaml`, use them everywhere (DB open, artifact writes, log files) instead of recomputing defaults.
