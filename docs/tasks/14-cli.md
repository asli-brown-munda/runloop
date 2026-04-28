# CLI

- [ ] **`runloop daemon start`** — Front the daemon launch through the CLI (delegating to `runloopd`) so users have a single entrypoint.
- [ ] **`runloop runs show <id>`** — Print run metadata, step statuses, durations, sink outputs, and recent log lines.
- [ ] **CLI output formats** — Add `--output json` to list/show commands so the CLI is scriptable.
- [ ] **Expand `internal/cli/commands_test.go`** — Cover the inbox add / list / show flow against an in-process test daemon to lock the JSON contracts.
