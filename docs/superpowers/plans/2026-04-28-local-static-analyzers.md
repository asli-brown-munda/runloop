# Local Static Analyzers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add local-only static analyzer tooling for the Go project.

**Architecture:** Keep analyzer integration at the repository tooling layer. `make lint` remains the main developer entry point and delegates to Go's built-in vet plus a pinned `golangci-lint` installation.

**Tech Stack:** Go 1.24, Make, `golangci-lint`.

---

## File Structure

- Create `.golangci.yml`: conservative `golangci-lint` configuration.
- Modify `Makefile`: add a pinned `GOLANGCI_LINT_VERSION`, `lint-install`, and a clearer `lint` target.
- Modify `README.md`: document installation and local usage.

### Task 1: Configure Local Analyzer

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Add conservative `golangci-lint` configuration**

Create `.golangci.yml`:

```yaml
version: "2"

run:
  timeout: 5m
  tests: true

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused

formatters:
  enable:
    - gofmt
    - goimports
```

- [ ] **Step 2: Validate configuration is accepted**

Run: `golangci-lint config verify`

Expected: command exits 0.

If `golangci-lint` is not installed, proceed to Task 2 first and return to this step.

### Task 2: Wire Makefile Commands

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add pinned analyzer version and phony target**

Change the first line to include `lint-install`, and add the version variable before targets:

```make
.PHONY: build test fmt lint lint-install run-daemon run-cli clean

GOLANGCI_LINT_VERSION ?= v2.8.0
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || printf '%s/bin/golangci-lint' "$$(go env GOPATH)")
```

- [ ] **Step 2: Replace `lint` with full local analyzer suite**

Replace the current `lint` target:

```make
lint:
	go vet ./...
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint is not installed. Run 'make lint-install'."; exit 1; }
	$(GOLANGCI_LINT) run ./...
```

- [ ] **Step 3: Add install helper**

Add after `lint`:

```make
lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
```

- [ ] **Step 4: Verify Makefile target discovery**

Run: `make -n lint`

Expected: output includes `go vet ./...`, the `command -v golangci-lint` check, and `golangci-lint run ./...`.

### Task 3: Document Local Usage

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update local development commands**

In the Common commands block, include:

```sh
go mod download
make lint-install
make test
make lint
make build
```

- [ ] **Step 2: Add short tooling note**

Add below the Common commands block:

```markdown
`make lint` runs local static analysis with `go vet` and `golangci-lint`. The `lint-install` target installs the pinned analyzer version used by this repository.
```

### Task 4: Verify and Fix Analyzer Findings

**Files:**
- Modify only files reported by the conservative analyzer configuration if needed.

- [ ] **Step 1: Format**

Run: `make fmt`

Expected: command exits 0.

- [ ] **Step 2: Test**

Run: `make test`

Expected: command exits 0.

- [ ] **Step 3: Lint**

Run: `make lint`

Expected: command exits 0. If the command fails because `golangci-lint` is missing, run `make lint-install` and retry. If it reports code findings, fix only those findings and rerun `make fmt`, `make test`, and `make lint`.
