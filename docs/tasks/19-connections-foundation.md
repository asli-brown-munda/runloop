# Connections Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this task. You are not alone in the codebase; do not revert unrelated edits, and keep ownership to the files below.

**Goal:** Add the shared connection model and resolver while preserving the existing secret resolver behavior.

**Architecture:** Keep `secrets.yaml` as the storage file. Add connection parsing and resolution to `internal/secrets` first, then optionally extract helpers into `internal/connections` only if the package grows too large. Existing callers of `secrets.Resolver` must continue to compile.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, existing file-backed secret resolver, table-driven tests.

---

## Context

Current `internal/secrets/service.go` supports:

- `secrets.<id>.file`
- `profiles.<profile>.env.<ENV>.secret`
- `Resolve(ctx, id)`
- `ResolveProfileEnv(ctx, profile, name)`
- `ProfileEnvConfigured(profile, name)`

The new user-facing model adds:

```yaml
connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: claude-api-key
```

## File Ownership

- Modify: `internal/secrets/service.go`
- Modify: `internal/secrets/service_test.go`
- Optional create: `internal/secrets/connections.go`
- Optional create: `internal/secrets/connections_test.go`

Do not edit GitHub source, Claude step, CLI, API, or docs in this task.

## Public Contract

Add these concepts to the secrets package:

```go
type ConnectionRef struct {
	Service string
	Name    string
}

type Connection struct {
	Service  string
	Name     string
	Provider string
}

type ConnectionEnvResolver interface {
	ResolveConnectionEnv(ctx context.Context, ref string, name string) (string, error)
	ConnectionConfigured(ref string) bool
	ListConnections() []Connection
	TestConnection(ctx context.Context, ref string) error
}

type TokenConnectionResolver interface {
	ResolveConnectionToken(ctx context.Context, ref string) (string, error)
}
```

Implement these methods on `FileResolver`:

- `ResolveConnectionToken(ctx, "github.work")`
- `ResolveConnectionEnv(ctx, "claude.default", "ANTHROPIC_API_KEY")`
- `ConnectionConfigured("github.work")`
- `ListConnections()`
- `TestConnection(ctx, "github.work")`

Validation defaults:

- Connection refs must be exactly `service.name`.
- Empty service or name is invalid.
- Missing connections return `connection "github.work" is not configured`.
- `static_token` requires `tokenSecret`.
- `env` requires the requested env key and its `secret`.
- Unknown providers return `connection "x.y" provider "..." is unsupported`.

## Tasks

- [ ] **Step 1: Add failing tests for parsing and static token resolution**

Add tests covering valid refs, invalid refs, missing connection, `static_token`, and `env` provider resolution. Use `t.TempDir()`, write protected secret files with `0600`, and verify existing `Resolve` still works.

Run:

```sh
go test ./internal/secrets -run 'TestFileResolver(Connection|Resolve)' -v
```

Expected before implementation: compile failure or failing tests for missing methods.

- [ ] **Step 2: Extend `fileConfig` and implement connection helpers**

Add `Connections map[string]map[string]connectionEntry` to the YAML config. Keep existing `Secrets` and `Profiles` structs unchanged. Implement ref parsing and provider dispatch with small helper functions.

- [ ] **Step 3: Preserve legacy behavior**

Run existing tests:

```sh
go test ./internal/secrets ./internal/steps ./internal/steps/claude
```

Expected: existing profile and secret tests still pass.

- [ ] **Step 4: Add list and health behavior**

`ListConnections()` should return stable sorted output by `service.name`. `TestConnection` should resolve the configured provider without returning secret values.

- [ ] **Step 5: Run verification**

```sh
go test ./internal/secrets ./...
```

Expected: all tests pass.

## Acceptance Criteria

- Existing secret IDs and `profiles.claude` continue to work.
- Connection methods expose no secret values.
- Static token and env providers work through the same file permission checks as current secrets.
- Errors use connection language, not only secret language, when resolving through a connection.

