# Manual GitHub Flow E2E

This runbook tests the GitHub PR source, unresolved review-thread normalization, workflow dispatch, `git_checkout`, Claude CLI execution, and markdown sink output.

It keeps Runloop config/state isolated with XDG directories while preserving your normal `HOME` so `auth: login` can reuse your local Claude CLI login.

## 1. Prerequisites

Install or verify:

```sh
go version
git --version
gh --version
jq --version
claude --version
```

Verify Claude login:

```sh
claude -p --permission-mode plan --max-budget-usd 0.05 --model sonnet \
  'Reply exactly: RUNLOOP_CLAUDE_READY'
```

Set GitHub variables:

```sh
export GITHUB_TOKEN='your-token-here'
export OWNER='asli-brown-munda'
export REPO='runloop'
```

The token must be able to read the repo, create branches/PRs, assign issues, and create PR review comments.

## 2. Create A Temporary PR With An Unresolved Review Thread

From the repository root:

```sh
bash docs/github-flow-create-test-pr.sh
export E2E_META="$(cat /tmp/runloop-github-e2e-latest)"
jq . "$E2E_META"
```

Important values:

```sh
export E2E_BRANCH="$(jq -r '.branch' "$E2E_META")"
export E2E_PR="$(jq -r '.prNumber' "$E2E_META")"
export E2E_HEAD_SHA="$(jq -r '.headSha' "$E2E_META")"
```

`E2E_HEAD_SHA` is the immutable PR snapshot. Runloop will pull `refs/pull/<number>/head` and verify that it resolves to this SHA.

## 3. Build And Initialize An Isolated Runloop Setup

```sh
make build

export RUNLOOP_E2E_ROOT="$(mktemp -d /tmp/runloop-manual-e2e.XXXXXX)"
export XDG_CONFIG_HOME="$RUNLOOP_E2E_ROOT/config-home"
export XDG_STATE_HOME="$RUNLOOP_E2E_ROOT/state-home"
export XDG_DATA_HOME="$RUNLOOP_E2E_ROOT/data-home"

./bin/runloop init
```

Optional but useful: write an env file so a second terminal can reuse the same setup.

```sh
cat > "$RUNLOOP_E2E_ROOT/env.sh" <<EOF
export RUNLOOP_E2E_ROOT="$RUNLOOP_E2E_ROOT"
export XDG_CONFIG_HOME="$XDG_CONFIG_HOME"
export XDG_STATE_HOME="$XDG_STATE_HOME"
export XDG_DATA_HOME="$XDG_DATA_HOME"
export E2E_META="$E2E_META"
export E2E_BRANCH="$E2E_BRANCH"
export E2E_PR="$E2E_PR"
export E2E_HEAD_SHA="$E2E_HEAD_SHA"
EOF
```

## 4. Configure Runloop Paths And Port

Use a non-default port if you might already have another Runloop daemon running.

```sh
export RUNLOOP_PORT="${RUNLOOP_PORT:-33156}"
export RUNLOOP_CONFIG_DIR="$XDG_CONFIG_HOME/runloop"

cat > "$RUNLOOP_CONFIG_DIR/config.yaml" <<EOF
daemon:
  bindAddress: 127.0.0.1
  port: $RUNLOOP_PORT
  stateDir: $XDG_STATE_HOME/runloop
  artifactDir: $XDG_DATA_HOME/runloop/artifacts
  logDir: $XDG_STATE_HOME/runloop/logs
sources:
  file: $RUNLOOP_CONFIG_DIR/sources.yaml
workflows:
  dir: $RUNLOOP_CONFIG_DIR/workflows
models: {}
EOF
```

If you created `env.sh`, append the port:

```sh
printf 'export RUNLOOP_PORT="%s"\n' "$RUNLOOP_PORT" >> "$RUNLOOP_E2E_ROOT/env.sh"
```

## 5. Configure GitHub Secret And Connection

```sh
mkdir -p "$RUNLOOP_CONFIG_DIR/secrets"
printf '%s\n' "$GITHUB_TOKEN" > "$RUNLOOP_CONFIG_DIR/secrets/github-work-token"
chmod 600 "$RUNLOOP_CONFIG_DIR/secrets/github-work-token"

cat > "$RUNLOOP_CONFIG_DIR/secrets.yaml" <<'EOF'
secrets:
  github-work-token:
    file: secrets/github-work-token

connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
EOF
```

## 6. Configure The GitHub PR Source

This query narrows the poller to the temporary PR branch:

```sh
cat > "$RUNLOOP_CONFIG_DIR/sources.yaml" <<EOF
sources:
  - id: manual
    type: manual
    enabled: true
  - id: github-assigned-prs
    type: github_pr
    enabled: true
    config:
      connection: github.work
      query: "repo:$OWNER/$REPO is:pr is:open head:$E2E_BRANCH"
      every: 30s
      pageSize: 10
EOF
```

## 7. Install The Workflow

Copy the smoke-test workflow:

```sh
cp docs/github-pr-claude-login-e2e.yaml \
  "$RUNLOOP_CONFIG_DIR/workflows/github-pr-claude-e2e.yaml"
```

That workflow contains two steps:

- `checkout`: fetches `{{ inbox.normalized.pullRef }}` and verifies `{{ inbox.normalized.headSHA }}`
- `agent`: runs Claude with local login auth, tools disabled, and a small budget cap

For a real "let Claude edit the checkout" workflow, remove `--tools ""`, remove the budget cap if needed, and switch `permissionMode` from `plan` to `acceptEdits`.

## 8. Start The Daemon

Terminal 1:

```sh
source "$RUNLOOP_E2E_ROOT/env.sh"
./bin/runloopd
```

The daemon performs one source sync immediately on startup. Later polls run every `30s`.

## 9. Verify In Another Terminal

Terminal 2:

```sh
source "$RUNLOOP_E2E_ROOT/env.sh"

./bin/runloop health | jq .
./bin/runloop sources list | jq .
./bin/runloop sources test github-assigned-prs | jq .
./bin/runloop workflows show github-pr-claude-e2e | jq '{definition, readiness, dispatches}'
./bin/runloop inbox list | jq .
./bin/runloop inbox show 1 | jq '{
  item,
  normalized: .version.Normalized,
  dispatches
}'
./bin/runloop runs list | jq .
./bin/runloop runs show 1 | jq .
```

Expected results:

- `sources test github-assigned-prs` returns `{"ok": true}`
- inbox item has `EntityType: github_pr_unresolved_review_threads`
- normalized payload includes one `unresolvedReviewThreadIDs` entry
- workflow dispatch status is `completed`
- run status is `completed`
- sink output points to `github-pr-claude-e2e.md`

## 10. Inspect Artifacts

```sh
export RUN_ARTIFACT_DIR="$XDG_DATA_HOME/runloop/artifacts/runs/run_1"

sed -n '1,200p' "$RUN_ARTIFACT_DIR/steps/checkout/git.log"
cat "$RUN_ARTIFACT_DIR/steps/checkout/output.json" | jq .
cat "$RUN_ARTIFACT_DIR/steps/agent/output.json" | jq .
cat "$RUN_ARTIFACT_DIR/steps/agent/stdout.log"
cat "$RUN_ARTIFACT_DIR/sinks/github-pr-claude-e2e.md"
```

Expected Claude marker:

```text
RUNLOOP_GITHUB_UNRESOLVED_E2E
```

## 11. Cleanup

Stop the daemon with `Ctrl-C`.

Then close the temporary PR, delete the temporary branch, and remove the local token file:

```sh
source "$RUNLOOP_E2E_ROOT/env.sh"
bash docs/github-flow-cleanup-test-pr.sh "$E2E_META"
rm -f "$XDG_CONFIG_HOME/runloop/secrets/github-work-token"
```

Verify cleanup:

```sh
export GH_TOKEN="$GITHUB_TOKEN"
gh api "repos/$OWNER/$REPO/pulls/$E2E_PR" --jq '{number, state, head:.head.ref, url:.html_url}'

if gh api "repos/$OWNER/$REPO/git/ref/heads/$E2E_BRANCH" >/tmp/runloop-ref-check.json 2>/tmp/runloop-ref-check.err; then
  cat /tmp/runloop-ref-check.json
else
  printf 'branch_missing\n'
fi

test ! -f "$XDG_CONFIG_HOME/runloop/secrets/github-work-token" && \
  printf 'temporary_token_file_removed\n'
```

## Notes

- `runloop sources test <id>` only validates connectivity. It does not create inbox items.
- There is currently no `runloop sources poll <id>` command. To force a real poll, restart the daemon.
- The GitHub source normalizes `repository.cloneURL` to SSH when GitHub returns an SSH URL. The smoke-test workflow uses HTTPS instead because public HTTPS checkout works without SSH setup.
