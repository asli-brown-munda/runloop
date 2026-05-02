# Connections For GitHub PR Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this task. You are not alone in the codebase; do not revert unrelated edits, and keep ownership to the files below.

**Goal:** Let `github_pr` sources authenticate with `config.connection: github.work` while preserving legacy `config.tokenSecret`.

**Architecture:** The GitHub source should depend on a small token resolver interface, not on concrete connection storage. At construction, accept either `connection` or `tokenSecret`; at sync/test time, resolve a token through the selected path.

**Tech Stack:** Go, existing `internal/sources` factory options, `httptest` GraphQL tests.

---

## Context

Current GitHub source behavior lives in `internal/sources/github/github.go`:

- `New` requires `config.tokenSecret`.
- `Sync` calls `s.secrets.Resolve(ctx, s.tokenSecret)`.
- `@me` is expanded by calling GitHub GraphQL `viewer`.
- `Test` resolves a token and checks viewer login.

Target config:

```yaml
sources:
  - id: github-work-prs
    type: github_pr
    enabled: true
    config:
      connection: github.work
      query: "is:pr is:open assignee:@me org:acme"
      every: 5m
```

Legacy config remains valid:

```yaml
config:
  tokenSecret: github-token
```

## File Ownership

- Modify: `internal/sources/github/github.go`
- Modify: `internal/sources/github/github_test.go`
- Modify only if required by compile errors: `internal/sources/factory_options_test.go`

Do not edit `internal/secrets`, CLI, Claude step, docs, or examples in this task.

## Expected Resolver Interface

This task assumes Task 19 adds an optional interface on the resolver:

```go
type TokenConnectionResolver interface {
	ResolveConnectionToken(ctx context.Context, ref string) (string, error)
}
```

The GitHub source can type-assert this interface from `opts.Secrets`.

## Tasks

- [ ] **Step 1: Add tests for connection auth**

Extend `github_test.go` fake secrets to implement both `Resolve` and `ResolveConnectionToken`.

Add a test that builds a source with:

```go
map[string]any{
	"connection": "github.work",
	"query": "is:pr is:open assignee:@me",
	"pageSize": 10,
	"graphqlURL": server.URL,
}
```

Assert the GraphQL request uses `Authorization: Bearer gh-work`.

Run:

```sh
go test ./internal/sources/github -run TestSyncUsesConnectionToken -v
```

Expected before implementation: source construction fails because `tokenSecret` is missing.

- [ ] **Step 2: Add config fields**

Add `connection string` to `Source`. In `New`, read `cfg["connection"]` and `cfg["tokenSecret"]`.

Validation:

- If both are empty: return `github_pr source "prs" requires config.connection or config.tokenSecret`.
- If both are set: return `github_pr source "prs" sets both config.connection and config.tokenSecret`.
- If `connection` is set and resolver does not implement `ResolveConnectionToken`: return a clear unsupported resolver error.

- [ ] **Step 3: Resolve token through connection or legacy secret**

Update `token(ctx)`:

- If `s.connection != ""`, call `ResolveConnectionToken(ctx, s.connection)`.
- Else call legacy `Resolve(ctx, s.tokenSecret)`.
- Preserve empty token checks.

- [ ] **Step 4: Preserve legacy test coverage**

Keep `TestNewRequiresTokenSecret` but update it to expect the new error for missing `connection` or `tokenSecret`. Keep the existing sync test using `tokenSecret`.

- [ ] **Step 5: Run verification**

```sh
go test ./internal/sources/github ./internal/sources ./...
```

Expected: all tests pass.

## Acceptance Criteria

- `config.connection` works for GitHub PR sources.
- `config.tokenSecret` remains supported.
- Setting both `connection` and `tokenSecret` fails fast.
- Error messages name the source ID and the connection ref where relevant.

