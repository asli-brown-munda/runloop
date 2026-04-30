# Current State

Runloop currently has a working development baseline for the manual workflow path and configured sources:

```text
manual inbox item -> trigger evaluation -> dispatch -> workflow run -> transform step -> markdown sink artifact
configured source -> inbox item -> trigger evaluation -> dispatch -> workflow run
```

The setup is intentionally small. It establishes the daemon, CLI, config paths, SQLite schema, workflow loading, workflow inspection and enablement controls, source loading, source change detection, trigger matching, dispatch processing, step execution, and artifact writing.

## Binaries

- Daemon: `runloopd`
- CLI: `runloop`

Build both with:

```sh
make build
```

The binaries are written to:

```text
bin/runloopd
bin/runloop
```

## Workflow Definition

The sample workflow definition lives in:

```text
examples/workflows/manual-hello.yaml
```

Current workflow:

```yaml
id: manual-hello
name: Manual Hello
enabled: true

triggers:
  - type: inbox
    source: manual
    entityType: manual_item
    policy: once_per_item

steps:
  - id: echo
    type: transform
    input:
      message: "{{ inbox.normalized.message }}"
    output:
      result: "Hello from Runloop: {{ input.message }}"

sinks:
  - type: markdown
    path: report.md
```

`runloop init` writes the same workflow to:

```text
~/.config/runloop/workflows/manual-hello.yaml
```

That init-time template is embedded in:

```text
internal/config/loader.go
```

The Go model for workflow YAML is defined in:

```text
internal/workflows/model.go
```

The parser and validator are in:

```text
internal/workflows/parser.go
internal/workflows/validator.go
```

## Workflow Persistence

Workflow metadata and immutable versions are stored in SQLite.

Schema tables:

```text
workflow_definitions
workflow_versions
```

The schema is defined in:

```text
internal/store/migrations.go
```

Workflow YAML is loaded and persisted by:

```text
internal/store/repositories.go
```

Important behavior:

- `workflow_definitions` stores the stable workflow identity, name, and enabled flag.
- `workflow_versions` stores immutable YAML versions.
- A changed workflow YAML creates a new workflow version.
- An unchanged workflow YAML does not create a duplicate version.
- Enabling or disabling a workflow updates only `workflow_definitions.enabled` and does not create a workflow version.

## Workflow Inspection And Management

Workflow listing, inspection, and enablement controls are exposed through the local API and CLI.

CLI commands:

```text
runloop workflows list
runloop workflows show <id>
runloop workflows enable <id>
runloop workflows disable <id>
```

API endpoints:

```text
GET  /api/workflows
GET  /api/workflows/{id}
POST /api/workflows/{id}/enable
POST /api/workflows/{id}/disable
```

`runloop workflows show <id>` prints the current workflow definition state, the latest stored workflow version, the persisted YAML for that version, and the 10 most recent dispatches for the workflow.

## Sources And Inbox State

Configured source support is loaded from:

```text
~/.config/runloop/sources.yaml
```

The path is controlled by `sources.file` in:

```text
~/.config/runloop/config.yaml
```

Source config parsing is in:

```text
internal/config/sources.go
```

The source interface is in:

```text
internal/sources/source.go
```

Current source contract:

```go
type Source interface {
	ID() string
	Type() string
	Sync(ctx context.Context, cursor Cursor) ([]InboxCandidate, Cursor, error)
	Test(ctx context.Context) error
}

type ChangeNotifier interface {
	WaitForChange(ctx context.Context) error
}
```

Source construction is registry based:

```text
internal/sources/factory.go
internal/sources/manager.go
```

Built-in source packages register themselves from `init` functions. The daemon imports the built-in source packages so their constructors are available:

```text
internal/sources/filesystem
internal/sources/schedule
```

Manual source support is in:

```text
internal/sources/manual/manual.go
```

The manual source is always available. If it is not present in `sources.yaml`, the daemon registers a default source with ID `manual`.

Filesystem source support is in:

```text
internal/sources/filesystem/filesystem.go
```

Filesystem source config:

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

The filesystem source uses `github.com/fsnotify/fsnotify` to wait for OS file events in one configured directory. Matching create, write, or rename events wake the source runner, which then scans the directory, filters files by `glob`, emits one inbox candidate per changed file, inlines UTF-8 content up to 64 KiB, and stores a cursor using the newest observed modification time.

Schedule source support is in:

```text
internal/sources/schedule/schedule.go
```

Schedule source config:

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

Schedule sources support exactly one of:

```text
every
cron
```

The first sync establishes a baseline timestamp. Later syncs emit synthetic `schedule_tick` inbox candidates for elapsed schedule times and persist the latest fired timestamp as the cursor.

The source runner is in:

```text
internal/daemon/sourcerunner.go
```

Current source runner behavior:

- ensures a row exists in `sources` for each registered source
- skips running `manual`
- performs one startup sync for each non-manual source
- waits on `ChangeNotifier` sources such as `filesystem` instead of polling them
- uses a 5-second ticker only for sources without change notifications, such as `schedule`
- loads and stores source cursors through `source_cursors`
- upserts emitted inbox candidates
- evaluates triggers for changed inbox versions
- drains queued workflow runs after matching trigger evaluation

Inbox models and normalization helpers are in:

```text
internal/inbox/model.go
internal/inbox/normalize.go
internal/inbox/service.go
```

Items currently enter the inbox through two paths:

- Manual submissions call `POST /api/inbox`, usually through `runloop inbox add`. The API builds a `manual.Candidate`, upserts it through the inbox service, and evaluates triggers only when the upsert creates a changed version.
- Configured non-manual sources are loaded from `sources.yaml` and run by `internal/daemon/sourcerunner.go`. Each source `Sync` returns `InboxCandidate` values. The runner upserts each candidate, evaluates triggers for changed versions, drains queued workflow runs, and persists the source cursor when it advances.

The built-in non-manual sources today are `filesystem` and `schedule`. The manual source is always available but is not polled by the source runner.

Important rule:

```text
Inbox/source state is separate from workflow execution state.
```

An `InboxItem` does not have workflow statuses such as processing, completed, or failed. Those statuses belong to dispatches, workflow runs, and step runs.

`InboxItem` carries two optional timestamps for user-driven state:

```text
archived_at  — set by ArchiveInboxItem
ignored_at   — set by IgnoreInboxItem
```

These are mutually independent and do not affect trigger evaluation or dispatch creation.

Inbox deduplication uses:

```text
source_id + external_id
```

If the raw or normalized payload changes, a new `inbox_item_versions` row is created.

Package-level tests for both rules live in:

```text
internal/inbox/service_test.go
```

The current in-process source extension point is compile-time registration. A new source type must be implemented in Go, call `sources.Register`, and be imported into the daemon binary. Users can add more instances of built-in source types by editing their local `sources.yaml` without modifying this repo. External tools can also submit inbox items under custom source IDs through the local API or CLI, but those custom IDs are not registered sources unless the binary imports a matching source implementation.

## Execution State

Dispatch state is represented by:

```text
internal/dispatch/model.go
```

Run state is represented by:

```text
internal/runs/model.go
```

Step result contracts are represented by:

```text
internal/steps/contract.go
```

The minimal run engine is in:

```text
internal/runs/engine.go
```

The current run engine:

- claims queued dispatches
- creates workflow runs
- executes steps sequentially
- writes step input/output artifacts
- renders sinks
- marks runs and dispatches completed or failed

## Trigger Evaluation

Trigger evaluation is in:

```text
internal/triggers/evaluator.go
internal/triggers/policies.go
```

Current supported policies:

```text
once_per_item
once_per_version
manual_only
```

For the sample workflow, a manual inbox item with entity type `manual_item` matches the `manual-hello` workflow and creates a queued dispatch.

## Local API And CLI

The local HTTP API is in:

```text
internal/web/server.go
internal/web/api.go
```

The CLI is in:

```text
internal/cli/client.go
internal/cli/commands.go
```

The daemon binds to:

```text
127.0.0.1:8765
```

The CLI uses the local API for normal operations. `runloop init` is the exception: it writes config, sample files, and the auth token directly.

Current source API routes:

```text
GET  /api/sources
POST /api/sources/{id}/test
```

Current source CLI commands:

```sh
runloop sources list
runloop sources test <id>
```

Current inbox API routes:

```text
GET  /api/inbox
POST /api/inbox
GET  /api/inbox/{id}
POST /api/inbox/{id}/archive
POST /api/inbox/{id}/ignore
```

`GET /api/inbox/{id}` returns an enriched response:

```json
{
  "item":       { ... },
  "version":    { "rawPayload": {...}, "normalized": {...}, ... },
  "dispatches": [
    { "dispatch": {...}, "run": {...} }
  ]
}
```

`run` is omitted if no workflow run has been created for that dispatch yet.

Current inbox CLI commands:

```sh
runloop inbox list
runloop inbox show <id>
runloop inbox add --source <id> --external-id <id> --title <title> --json <json>
runloop inbox archive <id>
runloop inbox ignore <id>
```

Manual inbox submissions can specify a custom source ID:

```sh
runloop inbox add --source external-system --external-id item-1 --title "External item" --json '{"message":"hello"}'
```

## Default Runtime Paths

```text
~/.config/runloop/config.yaml
~/.config/runloop/sources.yaml
~/.config/runloop/workflows/
~/.config/runloop/auth.token
~/.local/state/runloop/runloop.db
~/.local/state/runloop/logs/
~/.local/share/runloop/artifacts/
```

Artifact layout:

```text
artifacts/
  inbox/
    inbox_<id>/
      raw.json
      normalized.json
  runs/
    run_<id>/
      steps/
        <step_id>/
          input.json
          output.json
      sinks/
        report.md
```

## Manual End-To-End Test

From the repo root:

```sh
make build
```

Use an isolated temporary home:

```sh
tmp=$(mktemp -d)
HOME="$tmp" ./bin/runloop init
HOME="$tmp" ./bin/runloopd
```

Leave the daemon running. In another terminal, use the same `tmp` value:

```sh
HOME="$tmp" ./bin/runloop health

HOME="$tmp" ./bin/runloop inbox add \
  --source manual \
  --external-id test-1 \
  --title "First test" \
  --json '{"message":"hello"}'

HOME="$tmp" ./bin/runloop inbox list
HOME="$tmp" ./bin/runloop workflows list
HOME="$tmp" ./bin/runloop runs list
HOME="$tmp" ./bin/runloop sources list
HOME="$tmp" ./bin/runloop sources test manual
```

Expected result:

- the manual inbox item is created
- the `manual-hello` workflow trigger matches
- a workflow dispatch is created
- a workflow run is created
- the `echo` transform step runs
- the markdown sink writes `report.md`
- `runs list` shows a run with `Status` set to `completed`

Verify artifacts:

```sh
find "$tmp/.local/share/runloop/artifacts" -type f | sort
cat "$tmp/.local/share/runloop/artifacts/runs/run_1/sinks/report.md"
```

Expected files include:

```text
artifacts/inbox/inbox_1/raw.json
artifacts/inbox/inbox_1/normalized.json
artifacts/runs/run_1/steps/echo/input.json
artifacts/runs/run_1/steps/echo/output.json
artifacts/runs/run_1/sinks/report.md
```

The report should include:

```text
Hello from Runloop: hello
```

## Verification Commands

Current baseline checks:

```sh
make lint
make test
make build
./bin/runloopd --help
./bin/runloop --help
```

## Current Checkpoint

The latest committed source checkpoint is:

```text
93c62cf Source changes
```

The current working tree extends that checkpoint with:

- Filesystem watcher support through `fsnotify`
- `runloop inbox archive <id>` and `runloop inbox ignore <id>` CLI subcommands
- Enriched `GET /api/inbox/{id}` response (item + latest version payload + dispatches/runs)
- Inbox versioning contract tests in `internal/inbox/service_test.go`
