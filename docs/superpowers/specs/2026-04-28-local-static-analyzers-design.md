# Local Static Analyzers Design

## Goal

Improve local code quality checks for the Go project by integrating mainstream static analyzers into developer commands, without adding CI.

## Scope

This change is local-only. It updates repository configuration, local Makefile targets, scripts if useful, and README instructions. It does not add GitHub Actions or other remote CI automation.

## Analyzer Strategy

Use `golangci-lint` as the primary analyzer runner because it is the common Go aggregator for static analysis and linting. Keep `go vet ./...` as an explicit baseline check because it is part of the Go toolchain and already exists in the project.

The initial `golangci-lint` configuration should be conservative. It should focus on correctness and low-noise hygiene checks rather than broad style enforcement. This avoids a large unrelated cleanup in the current MVP codebase while still catching real issues.

## Developer Interface

`make lint` runs the complete local analyzer suite:

- `go vet ./...`
- `golangci-lint run ./...`

If `golangci-lint` is missing, `make lint` should fail with a clear installation message instead of a shell error.

Add `make lint-install` to install the pinned analyzer version through Go tooling.

## Documentation

Update the README local development section to include:

- installing the analyzer with `make lint-install`
- running `make lint`
- the fact that the analyzer integration is local developer tooling

## Verification

After implementation, run:

- `make fmt`
- `make test`
- `make lint`

Any analyzer findings introduced by the selected conservative configuration should be fixed directly. Avoid unrelated refactors.
