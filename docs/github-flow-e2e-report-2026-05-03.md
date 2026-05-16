# GitHub Flow E2E Report - 2026-05-03

## Summary

Result: success with the new GitHub token.

Validated live path:

```text
temporary GitHub PR + unresolved review thread
-> GitHub GraphQL source test
-> daemon startup poll
-> github_pr_unresolved_review_threads inbox version
-> workflow dispatch
-> git_checkout
-> Claude CLI step
-> markdown sink artifact
```

The earlier token only supported the read-only path and failed when creating temporary PR review-thread test data with `HTTP 403`. The new token successfully created the temporary PR and unresolved review thread needed for the full test.

## Environment

- Repository under test: `/home/shubham/work/opensource/runloop`
- Git remote: `https://github.com/asli-brown-munda/runloop.git`
- GitHub viewer resolved by token: `asli-brown-munda`
- Target repository: `asli-brown-munda/runloop`
- Temporary PR: `https://github.com/asli-brown-munda/runloop/pull/2`
- Temporary branch: `runloop-e2e-github-poll-20260503101401`
- Runloop isolated root: `/tmp/runloop-live-e2e-write-20260503101401`
- Daemon port: `33156`
- Runloop config/state/data isolation: `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and `XDG_DATA_HOME`
- Normal `HOME` preserved so `claude` could use the existing local Claude CLI login
- Temporary token file was removed after the run

## What Passed

- `make build` passed.
- The new token resolved viewer `asli-brown-munda` with `ADMIN` permission on `asli-brown-munda/runloop`.
- A temporary branch, commit, PR `#2`, assignment, and unresolved review comment were created through GitHub APIs.
- GraphQL verification showed one unresolved review thread on PR `#2`.
- `runloop health` returned `{"ok": true}`.
- `runloop sources test github-assigned-prs` returned `{"ok": true}` and called GitHub.
- The daemon startup poll created inbox item `1`:
  - source: `github-assigned-prs`
  - external ID: `asli-brown-munda/runloop#2`
  - entity type: `github_pr_unresolved_review_threads`
  - unresolved thread count: `1`
  - unresolved thread ID: `PRRT_kwDOSPGKTM5_Lcqg`
- Workflow `github-pr-claude-e2e` dispatched and completed run `1`.
- `git_checkout` fetched `refs/pull/2/head` over HTTPS and verified head SHA `a7081ba9a3ac6ffa8d19f8e067fb4b37e70e3748`.
- Claude step exit code was `0` with empty stderr.
- Claude stdout contained:

```text
RUNLOOP_GITHUB_UNRESOLVED_E2E validated: smoke-test thread observed on PR https://github.com/asli-brown-munda/runloop/pull/2 (Thread 1: PRRT_kwDOSPGKTM5_Lcqg, marker 20260503101401).
```

- Markdown sink was written at:

```text
/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/runs/run_1/sinks/github-pr-claude-e2e.md
```

## Cleanup

- The daemon was stopped.
- Temporary token file was removed from the isolated Runloop config.
- Temporary PR `#2` was closed.
- Temporary branch `runloop-e2e-github-poll-20260503101401` was deleted.
- Follow-up verification returned PR state `closed` and the branch ref was missing, as expected.

## Workflow Notes

The workflow used the same source entity type as the sample unresolved-review workflow, but used two local-test adaptations:

- Claude used `auth: login` with no `claude.default` connection, because the machine already had Claude CLI login state under the normal `HOME`.
- The checkout step used `{{ inbox.normalized.repository.url }}.git` instead of `{{ inbox.normalized.repository.cloneURL }}`. The GitHub source normalizes `cloneURL` to SSH when GitHub provides `sshUrl`; in this shell, `git ls-remote git@github.com:asli-brown-munda/runloop.git HEAD` failed with host-key/auth errors, while HTTPS access succeeded for the public repo.

## UX Findings

- XDG isolation worked well for Runloop state while preserving `HOME` for Claude CLI login. This is better than using a temporary `HOME` when testing `auth: login`.
- `sources test <id>` is useful for credential and viewer validation, but it does not create inbox items. A `sources sync <id>` or `sources poll <id>` command would make E2E testing much clearer.
- There is no force-poll endpoint; restarting the daemon is the practical way to force startup sync.
- The sample GitHub workflow assumes `connection: claude.default`. That is good for API-key auth, but a documented login-auth variant would make local Claude CLI smoke tests easier.
- The sample GitHub workflow also assumes SSH clone availability through `repository.cloneURL`. For public-repo smoke tests, HTTPS is much less fragile.
- `workflows show` returned `readiness: null` when there were no diagnostics. An explicit empty list would be easier to interpret in scripts.
- Secret setup is secure, but verbose. A documented "write this env var into a temporary static token connection" helper would reduce local-test friction.

## Evidence Files

- Inbox normalized payload: `/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/inbox/inbox_1/normalized.json`
- Git checkout log: `/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/runs/run_1/steps/checkout/git.log`
- Checkout output JSON: `/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/runs/run_1/steps/checkout/output.json`
- Claude stdout: `/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/runs/run_1/steps/agent/stdout.log`
- Claude output JSON: `/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/runs/run_1/steps/agent/output.json`
- Markdown sink: `/tmp/runloop-live-e2e-write-20260503101401/data-home/runloop/artifacts/runs/run_1/sinks/github-pr-claude-e2e.md`
- Daemon log: `/tmp/runloop-live-e2e-write-20260503101401/state-home/runloop/logs/runloopd.log`

## Verification Commands

```sh
make build
./bin/runloop health
./bin/runloop sources test github-assigned-prs
./bin/runloop workflows show github-pr-claude-e2e
./bin/runloop inbox list
./bin/runloop inbox show 1
./bin/runloop runs list
./bin/runloop runs show 1
make test
```
