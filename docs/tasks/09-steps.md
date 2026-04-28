# Steps

- [ ] **Shell step hardening** — Capture stdout and stderr separately in `internal/steps/shell.go`, persist them as artifacts, respect a per-step timeout, and gate execution on an explicit workflow permission flag.
- [ ] **Wait step** — Implement `internal/steps/wait.go` as a simple in-process timer, with a TODO/interface for durable resume across daemon restarts.
- [ ] **Transform step template safety** — Make `{{ ... }}` rendering robust to missing keys (return a typed error instead of `<no value>`) and add tests for nested paths like `inbox.normalized.message`.
- [ ] **Step result artifact contract** — Standardize how `StepResult.Artifacts` are persisted so future step types do not each invent their own layout.
