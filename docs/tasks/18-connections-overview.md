# Connections Overview

> **For agentic workers:** You are not alone in the codebase. Do not revert unrelated edits, and coordinate through the file ownership listed in each task file. Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` when implementing a task.

## Goal

Introduce one user-facing credential model: **connections**. Users set up `github.work`, `claude.default`, and similar connections once, then sources and workflow steps reference those connections without caring whether the underlying credential is a static API key, a refreshable OAuth token, a GitHub App token, or a command-produced token.

## Current State

- File-backed secrets live in `~/.config/runloop/secrets.yaml` and are resolved by `internal/secrets/service.go`.
- Existing `profiles` in `secrets.yaml` map environment variables to secret IDs for steps, especially `profiles.claude.env.ANTHROPIC_API_KEY`.
- `github_pr` sources currently require `config.tokenSecret` and resolve that secret directly.
- Claude steps currently use `auth: login`, `auth: apiKey`, or `auth: auto`, and API-key auth resolves `profiles.claude`.
- `runloop init` currently writes commented `secrets` and `profiles` examples, not `connections`.
- The repo already has source test commands: `runloop sources list` and `runloop sources test <id>`.

## Target UX

Preferred source config:

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

Preferred workflow step config:

```yaml
steps:
  - id: agent
    type: claude
    connection: claude.default
```

Preferred storage shape in `secrets.yaml` for the first implementation slice:

```yaml
secrets:
  github-work-token:
    file: secrets/github-work-token
  claude-api-key:
    file: secrets/claude-api-key

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

Keep `secrets` and legacy `profiles` supported as implementation details and backwards-compatible config, but docs should present `connections` first.

## Parallel Work Map

- Task 19, foundation, defines the shared connection resolver contract. It should be merged first if possible, but other workers can start from the contract documented there.
- Task 20 owns GitHub PR source integration and should avoid editing CLI, Claude, or docs except its tests.
- Task 21 owns Claude step integration and workflow step schema changes.
- Task 22 owns CLI/API commands for listing and testing connections.
- Task 23 owns refreshable GitHub auth providers and should not change source/step public config beyond the provider fields it needs.
- Task 24 owns docs, examples, and validation-oriented manual test docs.

## Compatibility Rules

- `config.tokenSecret` for GitHub PR sources remains valid.
- `profiles.claude` remains valid for Claude API-key auth.
- `auth: login`, `auth: apiKey`, and `auth: auto` remain valid.
- New `connection` fields should be additive.
- Error messages should mention the user-facing connection name when a connection was used.

## Verification Baseline

Run these before and after integrating each completed task:

```sh
go test ./...
go test ./internal/secrets ./internal/sources/github ./internal/steps/claude ./internal/steps ./internal/cli ./internal/web ./internal/config ./internal/workflows
```

