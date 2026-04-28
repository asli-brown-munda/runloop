# Current State

Runloop currently has a working development baseline for the minimal manual workflow path:

```text
manual inbox item -> trigger evaluation -> dispatch -> workflow run -> transform step -> markdown sink artifact
```

The setup is intentionally small. It establishes the daemon, CLI, config paths, SQLite schema, workflow loading, trigger matching, dispatch processing, step execution, and artifact writing.

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

## Source And Inbox State

Manual source support is in:

```text
internal/sources/manual/manual.go
```

The source interface is in:

```text
internal/sources/source.go
```

Inbox models and normalization helpers are in:

```text
internal/inbox/model.go
internal/inbox/normalize.go
internal/inbox/service.go
```

Important rule:

```text
Inbox/source state is separate from workflow execution state.
```

An `InboxItem` does not have workflow statuses such as processing, completed, or failed. Those statuses belong to dispatches, workflow runs, and step runs.

Inbox deduplication uses:

```text
source_id + external_id
```

If the raw or normalized payload changes, a new `inbox_item_versions` row is created.

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

## Current Commit

The initial scaffold checkpoint was committed as:

```text
6c45b79 feat: scaffold runloop mvp
```
