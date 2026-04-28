# Tooling And Quality

- [ ] **Wire golangci-lint into `make lint`** — Run `go vet ./...` and `golangci-lint run ./...` from `make lint`, fail with a clear install message if the binary is missing, and add `make lint-install` per `docs/superpowers/specs/2026-04-28-local-static-analyzers-design.md`.
- [ ] **Document local analyzer setup in README** — Add the `make lint-install` and `make lint` steps to the local development section of `README.md` and call out that this is local-only tooling.
- [ ] **Fix initial analyzer findings** — After enabling the conservative analyzer config, fix the issues that surface in the existing packages without unrelated refactors.
- [ ] **Add `make fmt-check`** — Non-mutating gofmt/goimports check that fails CI-style locally so contributors can verify formatting before committing.
- [ ] **Stabilize `scripts/dev.sh`** — Make the dev script idempotent (build, isolated `HOME`, init, run daemon) so the manual end-to-end demo in `docs/current_state.md` is one command.
