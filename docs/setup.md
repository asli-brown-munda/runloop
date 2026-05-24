# Setup Guide

Use this guide to set up Runloop from a fresh checkout, run the sample workflow, and optionally configure local credentials for GitHub and Claude.

## Prerequisites

- Go 1.24 or newer
- Git
- A POSIX-style shell
- Optional for GitHub workflows: a GitHub token that can read the repositories you want Runloop to inspect
- Optional for Claude workflows: Claude CLI on `PATH` and either Claude CLI login state or an Anthropic API key

## 1. Build Runloop

Run from the repository root:

```sh
go mod download
make build
```

This writes:

```text
bin/runloopd
bin/runloop
```

## 2. Start an Isolated Daemon

For local evaluation, use the included development script. It builds the binaries, initializes config if needed, and runs the daemon with `HOME` set to an isolated directory.

Terminal 1:

```sh
export RUNLOOP_DEV_HOME="${RUNLOOP_DEV_HOME:-$PWD/.runloop-dev-home}"
./scripts/dev.sh
```

The script prints the isolated home it is using. By default, that is:

```text
.runloop-dev-home
```

To use a different location:

```sh
export RUNLOOP_DEV_HOME=/tmp/runloop-demo
./scripts/dev.sh
```

Leave the daemon running and use the same `RUNLOOP_DEV_HOME` value in the second terminal.

## 3. Point the CLI at the Same Home

In another terminal from the repository root:

```sh
export RUNLOOP_DEV_HOME="${RUNLOOP_DEV_HOME:-$PWD/.runloop-dev-home}"
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop health
```

Expected output includes:

```json
{
  "ok": true
}
```

List the initialized resources:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop workflows list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop sources list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop connections list
```

Expected result:

- `workflows list` includes `manual-hello`.
- `sources list` includes `manual`.
- `connections list` succeeds even when no uncommented connections are configured.

## 4. Run the Sample Workflow

Add a manual inbox item:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop inbox add \
  --source manual \
  --external-id demo-1 \
  --title "Demo item" \
  --json '{"message":"hello"}'
```

Inspect the result:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop inbox list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop runs list
find "$RUNLOOP_DEV_HOME/.local/share/runloop/artifacts/runs" -path '*/sinks/report.md' -print
```

Expected result:

- `inbox list` shows the item you added.
- `runs list` shows a completed run for `manual-hello`.
- The `find` command prints a path ending in `sinks/report.md`.

Inspect the report:

```sh
report_path=$(find "$RUNLOOP_DEV_HOME/.local/share/runloop/artifacts/runs" -path '*/sinks/report.md' | sort | tail -n 1)
sed -n '1,120p' "$report_path"
```

Expected content includes:

```text
Hello from Runloop: hello
```

## 5. Know Where Files Live

In the isolated setup, Runloop writes under `$RUNLOOP_DEV_HOME`:

```text
.config/runloop/config.yaml
.config/runloop/sources.yaml
.config/runloop/secrets.yaml
.config/runloop/secrets/
.config/runloop/workflows/
.config/runloop/auth.token
.local/state/runloop/runloop.db
.local/state/runloop/logs/
.local/share/runloop/artifacts/
```

For normal use without the isolated `HOME`, the same paths live under your real home directory:

```text
~/.config/runloop
~/.local/state/runloop
~/.local/share/runloop
```

## 6. Configure Connections

Connections are the preferred credential model. Configure a connection once in `secrets.yaml`, then sources and workflow steps reference it by name.

Stop the daemon before editing `secrets.yaml`; the secrets resolver is loaded when the daemon starts.

### Claude API Key Connection

Set `ANTHROPIC_API_KEY` in your shell, then write it to a local secret file:

```sh
: "${ANTHROPIC_API_KEY:?set ANTHROPIC_API_KEY first}"
mkdir -p "$RUNLOOP_DEV_HOME/.config/runloop/secrets"
printf "%s\n" "$ANTHROPIC_API_KEY" > "$RUNLOOP_DEV_HOME/.config/runloop/secrets/anthropic-api-key"
chmod 600 "$RUNLOOP_DEV_HOME/.config/runloop/secrets/anthropic-api-key"
```

Edit `$RUNLOOP_DEV_HOME/.config/runloop/secrets.yaml` and add:

```yaml
secrets:
  anthropic-api-key:
    file: secrets/anthropic-api-key

connections:
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: anthropic-api-key
```

Restart the daemon, then test:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop connections list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop connections test claude.default
```

### GitHub Token Connection

Set `GITHUB_TOKEN` in your shell, then write it to a local secret file:

```sh
: "${GITHUB_TOKEN:?set GITHUB_TOKEN first}"
mkdir -p "$RUNLOOP_DEV_HOME/.config/runloop/secrets"
printf "%s\n" "$GITHUB_TOKEN" > "$RUNLOOP_DEV_HOME/.config/runloop/secrets/github-work-token"
chmod 600 "$RUNLOOP_DEV_HOME/.config/runloop/secrets/github-work-token"
```

Edit `$RUNLOOP_DEV_HOME/.config/runloop/secrets.yaml` and add:

```yaml
secrets:
  github-work-token:
    file: secrets/github-work-token

connections:
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
```

Restart the daemon, then test local secret resolution:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop connections list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop connections test github.work
```

If you configure both Claude and GitHub, keep both entries under the same top-level maps:

```yaml
secrets:
  anthropic-api-key:
    file: secrets/anthropic-api-key
  github-work-token:
    file: secrets/github-work-token

connections:
  claude:
    default:
      provider: env
      env:
        ANTHROPIC_API_KEY:
          secret: anthropic-api-key
  github:
    work:
      provider: static_token
      tokenSecret: github-work-token
```

To use the GitHub PR source, edit `$RUNLOOP_DEV_HOME/.config/runloop/sources.yaml` and add a `github_pr` source:

```yaml
sources:
  - id: manual
    type: manual
    enabled: true
  - id: github-assigned-prs
    type: github_pr
    enabled: true
    config:
      connection: github.work
      query: "repo:OWNER/REPO is:pr is:open assignee:@me"
      every: 5m
      pageSize: 50
```

Replace `OWNER/REPO` with a repository the token can read, restart the daemon, then test the remote source:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop sources test github-assigned-prs
```

`sources test github-assigned-prs` calls GitHub. It is useful for manual setup, not for automated tests.

## 7. Next Steps

- Author workflows in `$RUNLOOP_DEV_HOME/.config/runloop/workflows/`.
- Use [Workflow Authoring](workflows.md) for supported trigger, step, sink, and template fields.
- Use [Troubleshooting](troubleshooting.md) when the daemon, CLI, credentials, or workflow runs do not behave as expected.
- Use [Local Development Testing](local-development-testing.md) for deeper smoke tests and contributor verification.
