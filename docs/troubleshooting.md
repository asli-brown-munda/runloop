# Troubleshooting

Use this guide when the daemon, CLI, credentials, sources, or workflow runs do not behave as expected.

## CLI Cannot Reach the Daemon

Symptom:

```text
connection refused
```

Fix:

- Confirm the daemon is running.
- Confirm the CLI uses the same `HOME` as the daemon.

For the isolated setup:

```sh
export RUNLOOP_DEV_HOME="${RUNLOOP_DEV_HOME:-$PWD/.runloop-dev-home}"
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop health
```

If `./scripts/dev.sh` is running in another terminal, that health command should return `{"ok": true}`.

## Port 8765 Is Already In Use

Symptom:

```text
bind: address already in use
```

Fix:

1. Stop the other Runloop daemon if you do not need it.
2. Or use a different isolated home and set a different port in that home's config.

Example:

```sh
export RUNLOOP_DEV_HOME=/tmp/runloop-port-8766
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop init
cfg="$RUNLOOP_DEV_HOME/.config/runloop/config.yaml"
awk '{ sub("port: 8765", "port: 8766"); print }' "$cfg" > "$cfg.tmp" && mv "$cfg.tmp" "$cfg"
HOME="$RUNLOOP_DEV_HOME" ./bin/runloopd
```

Use the same `HOME` for CLI commands so the CLI reads the same port:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop health
```

## Unauthorized API Responses

Symptom:

```text
unauthorized
```

Fix:

- Confirm the daemon and CLI use the same `HOME`.
- Confirm `auth.token` exists in that home:

```sh
test -f "$RUNLOOP_DEV_HOME/.config/runloop/auth.token"
```

If this is only an isolated local setup, reinitialize a clean home:

```sh
rm -rf "$RUNLOOP_DEV_HOME"
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop init
```

Then restart the daemon.

## Config Fails to Load

Symptom:

```text
invalid config ... field ... not found
```

Fix:

- Check `$RUNLOOP_DEV_HOME/.config/runloop/config.yaml`.
- Remove misspelled or unsupported keys.
- Keep custom model-specific settings under `models`, which intentionally accepts arbitrary nested keys.

## Workflow Does Not Appear

Fix:

- Confirm the file is under the configured workflow directory.
- Confirm the extension is `.yaml` or `.yml`.
- Check daemon logs:

```sh
sed -n '1,200p' "$RUNLOOP_DEV_HOME/.local/state/runloop/logs/runloopd.log"
```

Common causes:

- Missing `id` or `name`.
- Missing `triggers` or `steps`.
- Duplicate step IDs.
- Unsupported step or sink type.
- Unknown trigger field.

## Workflow Appears but Does Not Run

Fix:

1. Confirm the workflow is enabled:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop workflows show <workflow-id>
```

2. Confirm the inbox item source and entity type match the trigger:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop inbox show <item-id>
```

3. Confirm the trigger policy is not `manual_only`.

For the sample workflow, the trigger expects:

```yaml
source: manual
entityType: manual_item
```

## Run Fails Before a Shell, Claude, or Git Checkout Step

Symptom:

```text
shell steps are disabled unless workflow permissions.shell is true
```

Fix:

Add shell permission to the workflow:

```yaml
permissions:
  shell: true
```

This permission is required for `shell`, `claude`, and `git_checkout` steps.

## Claude or Git Readiness Fails

Check readiness:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop workflows show <workflow-id>
```

Fixes:

- If `claude` is missing, install Claude CLI and ensure it is on `PATH`.
- If `git` is missing, install Git and ensure it is on `PATH`.
- If `auth: apiKey` is used, configure `connection: claude.default` or legacy `profiles.claude`.
- If relying on Claude CLI login, keep `auth: login` or `auth: auto` and make sure the daemon's `HOME` contains the CLI login state.

## Connection Test Fails

Fix:

- Confirm the connection exists in `secrets.yaml`.
- Confirm every referenced secret file exists under the Runloop config directory.
- Confirm secret file permissions are restrictive:

```sh
chmod 600 "$RUNLOOP_DEV_HOME/.config/runloop/secrets/"*
```

- Restart the daemon after editing `secrets.yaml`.

For local static-token connections, `connections test` validates local secret resolution. For GitHub user-device connections, it may refresh or validate the configured token file.

## GitHub Source Test Fails

Command:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop sources test github-assigned-prs
```

Fixes:

- Confirm `sources.yaml` uses `connection: github.work`.
- Confirm `connections test github.work` succeeds.
- Confirm the GitHub token can read the target repository and pull requests.
- Narrow the query while testing:

```yaml
query: "repo:OWNER/REPO is:pr is:open assignee:@me"
```

`sources test` calls GitHub. Do not include it in automated tests that must run without network credentials.

## Filesystem Source Does Not Emit Items

Fix:

- Confirm the configured directory exists before the daemon starts.
- Confirm the changed file matches the configured `glob`.
- Restart the daemon after editing `sources.yaml`.

Example:

```yaml
sources:
  - id: notes
    type: filesystem
    enabled: true
    config:
      directory: ~/runloop-inbox
      glob: "*.md"
```

## SQLite Database Is Locked

Symptom:

```text
database is locked
```

Fix:

- Run only one daemon against the same state directory.
- Stop extra daemons and retry.
- For isolated evaluation, use a fresh `RUNLOOP_DEV_HOME`.

Current MVP does not enforce a PID file or single-instance lock before startup.

## Stale Running Runs After a Crash

Current MVP recovery for interrupted runs is not implemented. If the daemon crashes during a run, old rows may remain in `running`.

For disposable local setup:

```sh
rm -rf "$RUNLOOP_DEV_HOME"
./scripts/dev.sh
```

For non-disposable state, keep the database and artifacts for inspection before deleting anything:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop runs list
find "$RUNLOOP_DEV_HOME/.local/share/runloop/artifacts/runs" -maxdepth 3 -type f | sort
```

Then decide whether to keep the state for debugging or start a new isolated home.

## Reset an Isolated Setup

Stop the daemon, then remove the isolated home:

```sh
rm -rf "${RUNLOOP_DEV_HOME:-$PWD/.runloop-dev-home}"
```

Start again:

```sh
./scripts/dev.sh
```
