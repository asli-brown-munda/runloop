# Connections Documentation And Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this task. You are not alone in the codebase; do not revert unrelated edits, and inspect existing uncommitted changes before editing examples or config docs.

**Goal:** Update user-facing docs and examples so connections are the preferred credential model and legacy secrets/profiles are documented as compatibility/advanced usage.

**Architecture:** Keep documentation aligned with the implemented slices. Do not document refreshable GitHub setup as complete until Task 23 lands. Prefer short examples that demonstrate the mental model over exhaustive secret internals.

**Tech Stack:** Markdown docs, YAML examples, existing local-development testing guide.

---

## Context

Docs currently describe:

- Source config at `~/.config/runloop/sources.yaml`.
- File-backed secrets and `profiles.claude` in `docs/current_state.md`.
- GitHub PR source using `tokenSecret`.
- Claude readiness using `profiles.claude`.

Target user language:

```text
Connect GitHub once.
Connect Claude once.
Sources and workflows reference the connection they need.
```

## File Ownership

- Modify: `README.md`
- Modify: `docs/current_state.md`
- Modify: `docs/local-development-testing.md`
- Modify: `docs/tasks/15-secrets.md`
- Modify: `examples/sources.github-pr.yaml`
- Modify: `examples/workflows/github-pr-claude.yaml`
- Modify only if needed: `internal/config/loader.go` sample comments
- Modify only if needed: `internal/config/paths_test.go`

There are currently uncommitted changes in `examples/sources.yaml`, `internal/config/loader.go`, `internal/config/paths_test.go`, `examples/sources.github-pr.yaml`, and `internal/config/examples_test.go`. Treat them as user or generated work; do not revert them.

## Tasks

- [ ] **Step 1: Update examples after implementation shape is known**

Change GitHub source examples from:

```yaml
config:
  tokenSecret: github-token
```

to:

```yaml
config:
  connection: github.work
```

Keep a short legacy note that `tokenSecret` remains accepted.

- [ ] **Step 2: Update Claude workflow examples**

Where the workflow intends API-key auth, prefer:

```yaml
connection: claude.default
```

Do not add a connection to examples that intentionally rely on Claude CLI login state.

- [ ] **Step 3: Update `runloop init` sample comments**

If Task 19 and Task 21 have landed, update the generated `secrets.yaml` comment to show:

```yaml
# connections:
#   claude:
#     default:
#       provider: env
#       env:
#         ANTHROPIC_API_KEY:
#           secret: anthropic-api-key
```

Retain a short note that raw `secrets` are still used for storage.

- [ ] **Step 4: Update local development testing**

Add a smoke test section:

```sh
HOME="$tmp" ./bin/runloop connections list
HOME="$tmp" ./bin/runloop connections test github.work
HOME="$tmp" ./bin/runloop sources test github-work-prs
```

Document expected behavior without requiring real GitHub credentials in automated tests.

- [ ] **Step 5: Update task docs**

In `docs/tasks/15-secrets.md`, reframe old credential-profile tasks as superseded by the connections layer where appropriate. Keep redaction and hardened backend tasks.

- [ ] **Step 6: Run verification**

```sh
go test ./internal/config ./internal/workflows ./...
```

Expected: tests pass, including tests that validate initial config examples.

## Acceptance Criteria

- New users see `connections` before `secrets` or `profiles`.
- Existing `tokenSecret` and `profiles.claude` users still have compatibility notes.
- Docs do not claim GitHub refreshable auth is available until Task 23 is merged.
- Example YAML matches parser-supported fields.

