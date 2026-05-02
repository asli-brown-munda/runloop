# Current State

Runloop currently has a working development baseline for the manual workflow path, configured sources, configured runtime paths, and local step execution:

```text
manual inbox item -> trigger evaluation -> dispatch -> workflow run -> transform step -> markdown sink artifact
configured source -> inbox item -> trigger evaluation -> dispatch -> workflow run
shell/claude step -> isolated env + per-run workspace + step artifacts
```

The setup is intentionally small. It establishes the daemon, CLI, config paths and overrides, SQLite schema, workflow loading, workflow inspection and enablement controls, source loading, source change detection, trigger matching, dispatch processing, step execution, file-backed secrets, connections, legacy credential profiles, and artifact writing.

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

Current validation rejects missing workflow IDs or names, workflows without triggers or steps, duplicate step IDs, unsupported step types, unsupported sink types, malformed step environment entries, and unknown trigger fields. Unknown trigger fields are reported with YAML line context so authoring mistakes point back to the source file.

## Config Loading

Default config paths are derived from `HOME` and the XDG environment variables:

```text
XDG_CONFIG_HOME -> ~/.config
XDG_STATE_HOME  -> ~/.local/state
XDG_DATA_HOME   -> ~/.local/share
```

The config schema is defined in:

```text
internal/config/schema.go
```

The config loader is in:

```text
internal/config/loader.go
```

Important behavior:

- Missing `config.yaml` returns defaults.
- Unknown top-level and nested config keys are rejected with a single-line error.
- `models` is intentionally open-ended and allows arbitrary model-specific keys.
- Empty daemon bind address, port, state dir, artifact dir, log dir, sources file, and workflow dir fields fall back to defaults.
- Explicit `daemon.stateDir`, `daemon.artifactDir`, and `daemon.logDir` are honored by the daemon for the database, artifacts, and runtime log file.

`runloop init` creates:

```text
~/.config/runloop/config.yaml
~/.config/runloop/sources.yaml
~/.config/runloop/secrets.yaml
~/.config/runloop/secrets/
~/.config/runloop/workflows/manual-hello.yaml
~/.config/runloop/auth.token
~/.local/state/runloop/
~/.local/state/runloop/logs/
~/.local/share/runloop/artifacts/
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

While the daemon is running, `internal/daemon/workflowwatcher.go` watches the configured workflow directory for `.yaml` and `.yml` create, write, or rename events. Changed workflow content is reloaded into `workflow_versions`; unchanged content is skipped by the existing content hash; invalid workflow edits are logged and do not stop the daemon or remove the previously stored version. Workflow file deletes are ignored in the MVP.

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
internal/sources/github
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

GitHub PR source support is in:

```text
internal/sources/github/github.go
```

GitHub PR source config:

```yaml
sources:
  - id: github-assigned-prs
    type: github_pr
    enabled: true
    config:
      connection: github.work
      query: "is:pr is:open assignee:@me"
      every: 5m
      pageSize: 50
```

The GitHub source uses the GraphQL API, resolves `connection` through `secrets.yaml`, expands `@me` to the authenticated viewer login, and emits one inbox candidate per PR. `github.work` can use the implemented `static_token` provider for local validation or the implemented `github_user_device` provider for refreshable GitHub user-device credentials. Legacy `tokenSecret` remains accepted for existing configs, but new configs should prefer `connection`. PRs with unresolved review threads use entity type `github_pr_unresolved_review_threads`; PRs without unresolved threads use `github_pr_review_clean`, allowing workflows to trigger only on actionable review feedback.

The source runner is in:

```text
internal/daemon/sourcerunner.go
```

Current source runner behavior:

- ensures a row exists in `sources` for each registered source
- skips running `manual`
- performs one startup sync for each non-manual source
- waits on `ChangeNotifier` sources such as `filesystem` and `github_pr` instead of polling them
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

The built-in non-manual sources today are `filesystem`, `schedule`, and `github_pr`. The manual source is always available but is not polled by the source runner.

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

## Step Execution

Step execution is registry based. The dispatcher in:

```text
internal/steps/executor.go
```

renders `step.Input` against the run context, then looks up a `Handler` for `step.Type` from the step registry defined in:

```text
internal/steps/registry.go
```

Built-in step packages register themselves from `init` functions. The daemon imports the built-in step packages so their handlers are available:

```text
internal/steps/transform
internal/steps/shell
internal/steps/wait
internal/steps/claude
internal/steps/gitcheckout
```

Each handler receives a `steps.Request` carrying the parsed step, the owning workflow, the rendered input, the templating context, minimal environment defaults, and the configured secret resolver. Handlers own their own policy: the shell and Claude handlers enforce `permissions.shell` themselves rather than the dispatcher gating them.

Shell-like steps do not inherit the daemon's full environment. They receive `PATH`, `HOME`, `USER`, and `TERM` when present, plus explicit `step.env` entries. Env entries can be literals, direct secret references, or legacy credential profile references from `~/.config/runloop/secrets.yaml`.

The run engine creates a per-run workspace under:

```text
artifacts/runs/run_<id>/workspace
```

That path is available to templates as:

```text
{{ runloop.workspace }}
```

Shell and Claude steps default to that workspace when `workdir` is unset. The `git_checkout` step checks out a PR head into that workspace, verifies the expected head SHA when provided, and returns the checkout path for later steps as `{{ steps.<id>.path }}`.

Connections, file-backed secrets, and legacy credential profiles are implemented in:

```text
internal/secrets/service.go
```

Connections and their backing storage are configured in:

```text
~/.config/runloop/secrets.yaml
```

Current connections and secrets behavior:

- connections are the preferred credential model for built-in integrations
- `static_token` connections resolve token-style credentials from a secret ID
- `env` connections resolve one or more environment variables from secret IDs
- `github_user_device` connections resolve GitHub user-device credentials from the configured token file
- secret IDs map to files relative to the Runloop config directory
- absolute paths and paths escaping the config directory are rejected
- secret files must not be group- or world-readable
- secret values are trimmed of trailing newlines on read
- `step.env` can use literal values, `{ secret: <id> }`, or `{ from: <profile.ENV_NAME> }`
- legacy credential profiles map environment variable names to secret IDs and remain accepted for compatibility

Example:

```yaml
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

secrets:
  anthropic-api-key:
    file: secrets/anthropic-api-key
  github-work-token:
    file: secrets/github-work-token
```

The Claude step runs the local Claude CLI through `internal/steps/claude`. It supports `auth: login`, `auth: apiKey`, and `auth: auto`. Login auth relies on Claude CLI state under `HOME`; API-key auth injects `ANTHROPIC_API_KEY` from `connection: claude.default` when configured. Without a step connection, `profiles.claude` remains accepted for existing configs. Auto auth uses a configured connection first, then a legacy profile when configured, and otherwise falls back to CLI login state. `runloop workflows show <id>` includes readiness diagnostics for local machine issues such as a missing Claude binary, missing `git` binary for `git_checkout`, or missing required API-key connection/profile.

The daemon also wires the step registry into workflow validation:

```go
workflows.StepTypeValidator = steps.IsRegistered
```

That hook lets `internal/workflows/validator.go` reject unsupported step types without creating an import cycle from `workflows` to `steps`.

The current in-process step extension point is compile-time registration. A new step type must be implemented in Go, call `steps.Register` from an `init` function, and be imported into the daemon binary. No edits to `executor.go` or `validator.go` are needed.

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

Current connection API routes:

```text
GET  /api/connections
POST /api/connections/{service.name}/test
```

Current connection CLI commands:

```sh
runloop connections list
runloop connections test <service.name>
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
~/.config/runloop/secrets.yaml
~/.config/runloop/secrets/
~/.config/runloop/workflows/
~/.config/runloop/auth.token
~/.local/state/runloop/runloop.db
~/.local/state/runloop/logs/
~/.local/share/runloop/artifacts/
```

`daemon.stateDir`, `daemon.artifactDir`, and `daemon.logDir` in `config.yaml` override the default runtime state, artifact, and log locations.

Artifact layout:

```text
artifacts/
  inbox/
    inbox_<id>/
      raw.json
      normalized.json
  runs/
    run_<id>/
      workspace/
      steps/
        <step_id>/
          input.json
          output.json
          stdout.log   # shell/claude steps
          stderr.log   # shell/claude steps
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
make fmt-check
make lint
make test
make build
./bin/runloopd --help
./bin/runloop --help
```

## Current Checkpoint

The current committed baseline includes:

- Filesystem watcher support through `fsnotify`
- XDG-style default paths and explicit daemon runtime path overrides
- Strict config loading for known config fields, with open-ended `models` config
- Initial `secrets.yaml` generation plus file-backed secrets, connections, and legacy credential profiles
- Runtime workflow YAML reloads through `internal/daemon/workflowwatcher.go`
- Workflow validation for duplicate step IDs, unsupported step and sink types, malformed step env values, and unknown trigger fields with line context
- `runloop inbox archive <id>` and `runloop inbox ignore <id>` CLI subcommands
- Enriched `GET /api/inbox/{id}` response (item + latest version payload + dispatches/runs)
- Shell and Claude step execution with a minimal inherited environment and per-run workspace
- Connections as the preferred credential model, with legacy `tokenSecret` and `profiles.claude` compatibility
- GitHub PR source support for unresolved review-thread workflows
- Git checkout step support for local PR workspace preparation
- Claude and git checkout readiness diagnostics in `runloop workflows show <id>`
- Inbox versioning contract tests in `internal/inbox/service_test.go`
- Local development testing guidance in `docs/local-development-testing.md`
