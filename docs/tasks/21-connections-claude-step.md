# Connections For Claude Step Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this task. You are not alone in the codebase; do not revert unrelated edits, and keep ownership to the files below.

**Goal:** Let Claude workflow steps use `connection: claude.default` for API-key env injection while preserving existing `auth` and `profiles.claude` behavior.

**Architecture:** Add a generic `connection` field to workflow steps. Claude uses it to resolve `ANTHROPIC_API_KEY` via the connection resolver. Existing `auth` remains the mode selector, with `connection` acting as the credential source when API-key auth is active.

**Tech Stack:** Go, workflow YAML parsing, step readiness diagnostics, Claude step tests.

---

## Context

Current Claude behavior:

- `workflows.Step` has `Auth string`, but no `Connection`.
- `claudeEnv` supports `login`, `apiKey`, and `auto`.
- API-key auth resolves `profiles.claude.env.ANTHROPIC_API_KEY`.
- Auto auth injects an API key if `profiles.claude` is configured; otherwise it relies on Claude CLI login state.

Target workflow:

```yaml
steps:
  - id: agent
    type: claude
    connection: claude.default
    prompt: "..."
```

Legacy workflow remains valid:

```yaml
steps:
  - id: agent
    type: claude
    auth: apiKey
```

## File Ownership

- Modify: `internal/workflows/model.go`
- Modify: `internal/workflows/parser_test.go` or `internal/workflows/env_test.go`
- Modify: `internal/steps/claude/claude.go`
- Modify: `internal/steps/claude/claude_test.go`
- Modify: `internal/steps/readiness.go`
- Modify: `internal/steps/readiness_test.go`

Do not edit GitHub source, CLI, API, docs, or examples in this task.

## Expected Resolver Interface

This task assumes Task 19 adds:

```go
type ConnectionEnvResolver interface {
	ResolveConnectionEnv(ctx context.Context, ref string, name string) (string, error)
	ConnectionConfigured(ref string) bool
}
```

Claude should type-assert this optional interface from `req.Secrets`.

## Tasks

- [ ] **Step 1: Add workflow model test for `connection`**

Add a parse/model test proving this YAML decodes:

```yaml
steps:
  - id: agent
    type: claude
    connection: claude.default
```

Expected parsed field:

```go
Step.Connection == "claude.default"
```

Run:

```sh
go test ./internal/workflows -run TestWorkflowStepConnection -v
```

Expected before implementation: compile failure or empty field.

- [ ] **Step 2: Add `Connection` to `workflows.Step`**

Add:

```go
Connection string `yaml:"connection" json:"connection"`
```

Do not change trigger field validation; this is a step field.

- [ ] **Step 3: Add Claude connection tests**

Add tests for:

- `connection: claude.default` with default `auth: auto` injects `ANTHROPIC_API_KEY`.
- `auth: login` with a connection does not inject `ANTHROPIC_API_KEY`.
- Missing connection resolver returns a clear error only when the connection is needed.

- [ ] **Step 4: Update `claudeEnv`**

Behavior:

- `auth: login`: never resolve or inject connection env.
- `auth: apiKey`: if `Step.Connection` is set, resolve `ANTHROPIC_API_KEY` from that connection; otherwise use legacy `profiles.claude`.
- `auth: auto`: if `Step.Connection` is set, resolve and inject from it; otherwise preserve current profile-or-login fallback.

- [ ] **Step 5: Update readiness diagnostics**

For Claude steps:

- `connection` missing or broken is an error when `auth: apiKey`.
- `connection` missing or broken is an error when explicitly set with `auth: auto`.
- No connection and no legacy profile in `auth: auto` remains a warning about relying on Claude CLI login.

- [ ] **Step 6: Run verification**

```sh
go test ./internal/workflows ./internal/steps/claude ./internal/steps ./...
```

Expected: all tests pass.

## Acceptance Criteria

- `connection: claude.default` works for Claude API-key env injection.
- Existing `profiles.claude` behavior remains supported.
- `auth: login` continues to bypass API-key injection.
- Readiness messages mention the connection ref when a connection is configured.

