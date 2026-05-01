# Local Development Testing

Use this guide to verify Runloop changes on a local machine. It covers the automated checks, an isolated daemon setup, manual smoke tests, and the coverage scenarios expected for core subsystems.

## Automated Checks

Run from the repository root:

```sh
make fmt-check
make lint
make test
make build
```

Notes:

- `make fmt-check` requires `goimports`.
- `make lint` runs `go vet ./...` and `golangci-lint run ./...`.
- `make build` writes `bin/runloopd` and `bin/runloop`.

If `golangci-lint` is missing:

```sh
make lint-install
```

## Isolated Daemon Setup

Use a temporary `HOME` so local testing does not touch your real Runloop config, database, logs, or artifacts.

Terminal 1:

```sh
make build
tmp=$(mktemp -d)
HOME="$tmp" ./bin/runloop init
HOME="$tmp" ./bin/runloopd
```

Terminal 2, using the same `tmp` value:

```sh
HOME="$tmp" ./bin/runloop health
HOME="$tmp" ./bin/runloop workflows list
HOME="$tmp" ./bin/runloop sources list
HOME="$tmp" ./bin/runloop runs list
```

Expected result:

- `health` returns `{"ok": true}`.
- `workflows list` includes `manual-hello`.
- `sources list` includes `manual`.
- `runs list` succeeds, even when empty.

Stop the daemon with `Ctrl-C` when finished.

## Manual Inbox Workflow Smoke Test

With the isolated daemon running:

```sh
HOME="$tmp" ./bin/runloop inbox add --source manual --external-id t1 --title "T1" --json '{"message":"hi"}'
HOME="$tmp" ./bin/runloop inbox list
HOME="$tmp" ./bin/runloop inbox show 1
HOME="$tmp" ./bin/runloop runs list
```

Expected result:

- The first `inbox add` creates an inbox item and an inbox item version.
- `inbox show 1` returns the item, latest version, dispatches, and related run data when a run exists.
- `runs list` includes a completed run for the `manual-hello` workflow.
- The markdown sink writes a report artifact.

Archive and ignore commands:

```sh
HOME="$tmp" ./bin/runloop inbox archive 1
HOME="$tmp" ./bin/runloop inbox ignore 1
HOME="$tmp" ./bin/runloop inbox list
```

Expected result:

- `archive 1` returns `{"ok": true}` and populates `archived_at`.
- `ignore 1` returns `{"ok": true}` and populates `ignored_at`.
- Archive and ignore metadata do not change dispatch or run status.

## Workflow Management Smoke Test

With the isolated daemon running:

```sh
HOME="$tmp" ./bin/runloop workflows show manual-hello
HOME="$tmp" ./bin/runloop workflows disable manual-hello
HOME="$tmp" ./bin/runloop workflows list
HOME="$tmp" ./bin/runloop workflows enable manual-hello
HOME="$tmp" ./bin/runloop workflows show manual-hello
```

Expected result:

- `show manual-hello` prints the current definition state, latest stored version, persisted YAML, and recent dispatches.
- `disable manual-hello` sets `enabled` to `false`.
- `enable manual-hello` sets `enabled` to `true`.
- Enable and disable operations update `workflow_definitions.enabled` without creating new workflow versions.

## Workflow Reload Smoke Test

With the isolated daemon running, edit the initialized workflow file:

```sh
sed -i 's/Hello from Runloop:/Hello after reload:/' "$tmp/.config/runloop/workflows/manual-hello.yaml"
HOME="$tmp" ./bin/runloop workflows show manual-hello
```

Expected result:

- The daemon reloads the changed `.yaml` file without restart.
- `workflows show manual-hello` reports a newer workflow version and shows the changed YAML.
- Writing the same file content again does not create another workflow version.
- An invalid workflow edit is logged by the daemon and does not remove the previously stored valid version.

## Filesystem Source Smoke Test

Edit `$tmp/.config/runloop/sources.yaml`:

```yaml
sources:
  - id: notes
    type: filesystem
    enabled: true
    config:
      directory: ~/runloop-inbox
      glob: "*.md"
      entityType: note
```

Restart the daemon after editing the source config.

Terminal 1:

```sh
HOME="$tmp" mkdir -p "$tmp/runloop-inbox"
HOME="$tmp" ./bin/runloopd
```

Terminal 2:

```sh
HOME="$tmp" ./bin/runloop sources list
HOME="$tmp" ./bin/runloop sources test notes
HOME="$tmp" sh -c 'printf "hello\n" > "$HOME/runloop-inbox/one.md"'
HOME="$tmp" ./bin/runloop inbox list
```

Expected result:

- `sources list` includes `notes`.
- `sources test notes` returns `{"ok": true}`.
- Creating or updating `one.md` creates or updates a matching inbox item.
- Files that do not match `*.md` are ignored.

## Schedule Source Smoke Test

Edit `$tmp/.config/runloop/sources.yaml`:

```yaml
sources:
  - id: heartbeat
    type: schedule
    enabled: true
    config:
      every: 1m
      payload:
        reason: periodic
```

Expected result:

- The first sync establishes a cursor baseline.
- Later elapsed intervals emit `schedule_tick` inbox candidates.
- The source cursor advances after emitted ticks.

## Artifact Check

After a manual inbox run completes:

```sh
find "$tmp/.local/share/runloop/artifacts" -maxdepth 5 -type f | sort
```

Expected layout includes:

```text
.../artifacts/runs/run_<id>/sinks/report.md
```

Inspect a report:

```sh
sed -n '1,120p' "$tmp/.local/share/runloop/artifacts/runs/run_1/sinks/report.md"
```

Expected content includes the transform result from the manual workflow.

## Coverage Scenarios

Use this checklist when deciding whether a change has enough test or smoke coverage.

### Config And Startup

- `runloop init` creates config, sources file, workflow file, auth token, state directories, log directory, and artifact directory.
- `runloopd` starts from a clean temporary `HOME`.
- `runloop health` succeeds against the local daemon.
- CLI and daemon use the same bind address, port, and auth token from the temporary config.

### Workflow Loading And Persistence

- Loading a new workflow creates one `workflow_definitions` row and one `workflow_versions` row.
- Loading unchanged workflow YAML does not create duplicate versions.
- Loading changed workflow YAML creates a new immutable workflow version.
- Runtime edits to `.yaml` or `.yml` files in the workflow directory reload without restarting the daemon.
- Invalid runtime workflow edits are logged and leave the previous valid workflow version in place.
- Enabling or disabling a workflow updates only `workflow_definitions.enabled`.
- Disabled workflows are excluded from trigger evaluation.
- Unknown workflow IDs return a not-found error through workflow-specific API endpoints.
- Workflow validation rejects duplicate step IDs, unsupported step types, unsupported sink types, and unknown trigger fields with YAML line context.

### Workflow CLI And API

- `runloop workflows list` shows known workflows and enabled state.
- `runloop workflows show <id>` includes definition state, latest version, persisted YAML, and 10 recent dispatches.
- `runloop workflows disable <id>` disables the workflow through the API.
- `runloop workflows enable <id>` enables the workflow through the API.
- CLI workflow commands pretty-print JSON responses consistently with other Runloop commands.

### Inbox

- Manual `inbox add` creates a new item and version for a new source and external ID pair.
- Re-adding an identical payload for the same source and external ID does not create a new version.
- Re-adding a changed payload for the same source and external ID creates the next version.
- `inbox show <id>` returns the item, latest version, dispatches, and related runs.
- `inbox archive <id>` sets `archived_at`.
- `inbox ignore <id>` sets `ignored_at`.
- Archive and ignore metadata do not mutate dispatch or run state.

### Sources

- Manual source is available even when omitted from `sources.yaml`.
- Filesystem source registers from `sources.yaml`, validates its directory, and emits candidates for matching file changes.
- Filesystem source ignores non-matching glob entries.
- Schedule source accepts exactly one of `every` or `cron`.
- Schedule source establishes a baseline before emitting later ticks.
- Source cursors round trip through `source_cursors` and advance after successful sync.

### Trigger Evaluation And Dispatch

- Enabled inbox triggers match source and entity type.
- `once_per_item` creates only one dispatch per inbox item and workflow.
- `once_per_version` creates only one dispatch per inbox version and workflow version.
- Non-matching triggers record evaluations without creating dispatches.
- Queued dispatches can be claimed and moved through run execution.

### Run Engine, Steps, And Sinks

- A queued dispatch creates one workflow run.
- Built-in step types `transform`, `shell`, and `wait` self-register through `internal/steps/registry.go`; the executor dispatches by `step.Type` and the workflow validator rejects step types that are not registered.
- Transform steps receive inbox context and previous step output where applicable.
- Shell steps fail unless `permissions.shell` is enabled (the shell handler enforces the gate itself).
- Wait steps honor configured duration behavior.
- Failed steps mark the run and dispatch failed.
- Completed runs write sink outputs and artifact records.
- Markdown sink writes the expected report under the run artifact directory.

### Local API

- `/api/health` works without a token.
- Protected endpoints reject missing or invalid bearer tokens when an auth token exists.
- Inbox, workflow, source, and run endpoints return stable JSON shapes for CLI consumption.
- Invalid numeric IDs return a client error.
- Missing records return not found where endpoint semantics are record-specific.

### CLI

- Root and daemon help text use Runloop branding.
- CLI loads daemon host, port, and auth token from the default config paths.
- CLI surfaces non-2xx API responses as command errors.
- Commands that mutate state use POST endpoints.
- Commands that inspect state use GET endpoints.

## Troubleshooting

- If the CLI cannot reach the daemon, confirm `HOME="$tmp"` is set in both terminals.
- If protected endpoints return `unauthorized`, confirm the CLI and daemon are using the same temporary config directory and auth token.
- If filesystem changes are not detected, confirm the source directory exists before the daemon starts and that the changed file matches the configured glob.
- If no workflow dispatch is created, confirm the workflow is enabled and the trigger source/entity type matches the inbox item.
- If workflow edits do not reload, confirm the file extension is `.yaml` or `.yml` and check daemon logs for validation errors.
