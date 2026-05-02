# Connections CLI And API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this task. You are not alone in the codebase; do not revert unrelated edits, and keep ownership to the files below.

**Goal:** Add basic connection inspection and testing commands without building a full interactive setup wizard.

**Architecture:** Expose daemon-side connection list/test endpoints and thin CLI commands over the existing local API client. Keep mutation limited to a remove command only if the resolver exposes a safe file update helper; otherwise leave remove for a later task.

**Tech Stack:** Go, chi routes, existing CLI client, JSON output.

---

## Context

Current API and CLI support:

- `GET /api/sources`
- `POST /api/sources/{id}/test`
- `runloop sources list`
- `runloop sources test <id>`

Connection UX should start similarly:

```sh
runloop connections list
runloop connections test github.work
```

This task should not implement GitHub OAuth/device setup. That is Task 23.

## File Ownership

- Modify: `internal/web/api.go`
- Modify: `internal/web/api_test.go`
- Modify: `internal/web/server.go`
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`
- Optional modify: `internal/secrets/service.go` only for a tiny remove/list support method if Task 19 has not already added it.

Do not edit GitHub source, Claude step, docs, examples, or refreshable auth provider code in this task.

## Expected Resolver Interface

This task assumes Task 19 adds:

```go
type Connection struct {
	Service  string
	Name     string
	Provider string
}

type ConnectionInspector interface {
	ListConnections() []Connection
	TestConnection(ctx context.Context, ref string) error
}
```

If the actual Task 19 names differ, adapt only this task's call sites to the merged interface.

## Tasks

- [ ] **Step 1: Add API tests**

Add tests for:

- `GET /api/connections` returns configured connection names and providers, not secret values.
- `POST /api/connections/github.work/test` returns `{"ok": true}` when the resolver test passes.
- Missing connection returns a non-2xx response with a helpful message.

Run:

```sh
go test ./internal/web -run TestConnectionsAPI -v
```

Expected before implementation: route not found.

- [ ] **Step 2: Wire API routes**

Add:

```text
GET  /api/connections
POST /api/connections/{ref}/test
```

Response shape:

```json
[
  {"service":"github","name":"work","ref":"github.work","provider":"static_token"}
]
```

Do not include token values, secret IDs, refresh tokens, or file paths.

- [ ] **Step 3: Add CLI tests**

Add help/output tests proving `runloop connections --help` includes `list` and `test`.

If the repo has no HTTP client test harness for command execution, keep command tests to help text and rely on API tests for behavior.

- [ ] **Step 4: Add CLI commands**

Add `connectionsCommand()` to root command:

```text
runloop connections list
runloop connections test <service.name>
```

Use the existing `NewClient()` pattern and `printJSON`.

- [ ] **Step 5: Run verification**

```sh
go test ./internal/web ./internal/cli ./...
```

Expected: all tests pass.

## Acceptance Criteria

- Users can inspect available connections through CLI and API.
- Users can test a specific connection through CLI and API.
- No endpoint or command prints secret values.
- Commands work consistently with the existing source CLI style.

