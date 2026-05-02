# Refreshable GitHub Connections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this task. You are not alone in the codebase; do not revert unrelated edits, and keep ownership to the files below.

**Goal:** Add a GitHub connection provider that refreshes short-lived GitHub user credentials so users set up GitHub once and do not manually rotate token files.

**Architecture:** Add a provider behind the connection resolver, not inside the GitHub source. The GitHub source continues to request `ResolveConnectionToken(ctx, "github.work")`; the provider handles token refresh, persistence, and expiry.

**Tech Stack:** Go, `net/http`, protected local secret files, GitHub OAuth/device-flow compatible token shape, `httptest`.

---

## Context

This task depends on Task 19's connection resolver and Task 20's GitHub source support for `connection`.

Current static connection example:

```yaml
connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
```

Target refreshable example:

```yaml
connections:
  github:
    work:
      provider: github_user_device
      clientID: "<github-oauth-app-client-id>"
      tokenFile: secrets/github-work-oauth.json
```

Stored token file should be `0600` and contain:

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_at": "2026-05-02T12:00:00Z",
  "refresh_token_expires_at": "2026-11-02T12:00:00Z"
}
```

## File Ownership

- Modify: `internal/secrets/service.go`
- Create optional focused file: `internal/secrets/github_provider.go`
- Create optional focused test file: `internal/secrets/github_provider_test.go`
- Modify only if needed for retry: `internal/sources/github/github.go`
- Modify only if needed for retry tests: `internal/sources/github/github_test.go`

Do not edit CLI setup commands, Claude step, docs, or examples in this task.

## Tasks

- [ ] **Step 1: Add provider tests with fake GitHub token endpoint**

Use `httptest.Server` for token refresh. Test:

- Non-expired access token is returned without HTTP refresh.
- Token expiring within 2 minutes is refreshed.
- Refreshed token file is written with `0600`.
- Expired refresh token returns a user-facing reconnect error.

Run:

```sh
go test ./internal/secrets -run TestGitHubUserDeviceProvider -v
```

Expected before implementation: provider unsupported.

- [ ] **Step 2: Implement token file parsing and path safety**

Reuse the existing secret path rules:

- `tokenFile` must be relative to config dir.
- Absolute paths are rejected.
- Paths escaping config dir are rejected.
- Token file must not be group- or world-readable.

- [ ] **Step 3: Implement refresh behavior**

Provider behavior:

- Read token JSON.
- If `expires_at` is more than 2 minutes in the future, return `access_token`.
- Otherwise POST refresh request to GitHub token endpoint.
- Persist new token response before returning the new access token.
- Return a reconnect error if refresh token is expired or refresh fails with invalid grant.

Make the GitHub token endpoint configurable in tests, but default to GitHub's public endpoint in production.

- [ ] **Step 4: Add optional GitHub source retry**

If a GraphQL request returns 401 or 403 after using a connection token, invalidate or force-refresh if the provider exposes that hook, then retry once. If Task 19 does not expose an invalidation hook, skip retry and rely on pre-expiry refresh.

- [ ] **Step 5: Run verification**

```sh
go test ./internal/secrets ./internal/sources/github ./...
```

Expected: all tests pass.

## Acceptance Criteria

- `provider: github_user_device` resolves a valid access token without manual token replacement.
- Expired access tokens refresh automatically before use.
- Refresh token expiration produces a clear reconnect-required error.
- Token JSON is never logged or returned through API responses.

