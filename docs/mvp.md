# Project: Run Enclave

We are building the MVP of Run Enclave: a local-first agentic workflow executor daemon for developers.

The product is a Go-based background daemon that polls sources, puts normalized source entities into an Inbox, evaluates workflow YAML triggers, creates dispatches, runs workflow steps, stores artifacts/logs, and exposes a local CLI/API for inspection.

Do not overbuild. This setup task should create the initial repo, skeleton modules, basic interfaces, SQLite schema, config loading, and a minimal end-to-end path.

## Naming

Repository name:

run-enclave

Go module placeholder:

github.com/shubham-sharma/run-enclave

Daemon binary:

runenclaved

CLI binary:

runenclave

Main config directory:

~/.config/run-enclave/

State directory:

~/.local/state/run-enclave/

Artifact directory:

~/.local/share/run-enclave/artifacts/

Local API bind:

127.0.0.1:8765

## Core MVP rule

Keep source state separate from workflow execution state.

Inbox state is source state.
WorkflowDispatch, WorkflowRun, and StepRun are execution state.

An InboxItem must not have statuses like processing/completed/failed. Those belong to dispatches, runs, and steps.

## Initial repository structure

Create this structure:

```txt
run-enclave/
├── cmd/
│   ├── runenclaved/
│   │   └── main.go
│   └── runenclave/
│       └── main.go
│
├── internal/
│   ├── daemon/
│   │   ├── daemon.go
│   │   ├── supervisor.go
│   │   └── lifecycle.go
│   │
│   ├── config/
│   │   ├── loader.go
│   │   ├── paths.go
│   │   └── schema.go
│   │
│   ├── store/
│   │   ├── sqlite.go
│   │   ├── migrations.go
│   │   └── repositories.go
│   │
│   ├── sources/
│   │   ├── source.go
│   │   ├── manager.go
│   │   ├── manual/
│   │   │   └── manual.go
│   │   ├── filesystem/
│   │   │   └── filesystem.go
│   │   └── schedule/
│   │       └── schedule.go
│   │
│   ├── inbox/
│   │   ├── model.go
│   │   ├── service.go
│   │   └── normalize.go
│   │
│   ├── workflows/
│   │   ├── model.go
│   │   ├── parser.go
│   │   ├── registry.go
│   │   └── validator.go
│   │
│   ├── triggers/
│   │   ├── evaluator.go
│   │   └── policies.go
│   │
│   ├── dispatch/
│   │   ├── model.go
│   │   └── queue.go
│   │
│   ├── runs/
│   │   ├── model.go
│   │   ├── engine.go
│   │   └── recovery.go
│   │
│   ├── steps/
│   │   ├── contract.go
│   │   ├── executor.go
│   │   ├── shell.go
│   │   ├── transform.go
│   │   └── wait.go
│   │
│   ├── retry/
│   │   ├── policy.go
│   │   └── backoff.go
│   │
│   ├── artifacts/
│   │   ├── store.go
│   │   └── paths.go
│   │
│   ├── secrets/
│   │   ├── service.go
│   │   └── redactor.go
│   │
│   ├── web/
│   │   ├── server.go
│   │   └── api.go
│   │
│   └── cli/
│       ├── client.go
│       └── commands.go
│
├── examples/
│   ├── config.yaml
│   ├── sources.yaml
│   └── workflows/
│       └── manual-hello.yaml
│
├── docs/
│   ├── architecture.md
│   └── mvp.md
│
├── scripts/
│   ├── dev.sh
│   └── test.sh
│
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
````

## Dependencies

Use a small dependency set:

* cobra for CLI commands
* yaml.v3 for YAML parsing
* chi for local HTTP routing
* modernc.org/sqlite for cgo-free SQLite
* zerolog or slog; prefer standard library slog unless there is a strong reason otherwise
* expr-lang/expr only if needed for trigger expressions; otherwise stub expression matching first

Avoid adding UI dependencies in the initial setup.

## Phase 1: Bootstrap Go repo

Create:

* go.mod
* Makefile
* README.md
* .gitignore
* scripts/dev.sh
* scripts/test.sh

Makefile targets:

```makefile
build
test
fmt
lint
run-daemon
run-cli
clean
```

Initial commands should work:

```bash
make build
make test
./bin/runenclaved --help
./bin/runenclave --help
```

## Phase 2: Config and paths

Implement XDG-style default paths:

```txt
~/.config/run-enclave/config.yaml
~/.config/run-enclave/sources.yaml
~/.config/run-enclave/workflows/
~/.local/state/run-enclave/run-enclave.db
~/.local/share/run-enclave/artifacts/
~/.local/state/run-enclave/logs/
```

Create config structs:

```go
type Config struct {
    Daemon    DaemonConfig    `yaml:"daemon"`
    Sources   SourcesConfig   `yaml:"sources"`
    Workflows WorkflowsConfig `yaml:"workflows"`
    Models    ModelsConfig    `yaml:"models"`
}

type DaemonConfig struct {
    BindAddress string `yaml:"bindAddress"`
    Port        int    `yaml:"port"`
    StateDir    string `yaml:"stateDir"`
    ArtifactDir string `yaml:"artifactDir"`
    LogDir      string `yaml:"logDir"`
}
```

Provide defaults if config does not exist.

Add command:

```bash
runenclave init
```

This should create the config directories, sample config, sample sources file, and sample workflow.

## Phase 3: SQLite schema

Implement migrations in Go for MVP tables.

Create these tables:

```txt
sources
source_cursors
inbox_items
inbox_item_versions
workflow_definitions
workflow_versions
trigger_evaluations
workflow_dispatches
workflow_runs
step_runs
retry_attempts
artifacts
sink_outputs
daemon_events
secret_metadata
```

Do not implement every repository method yet. Add enough repository methods for the first end-to-end manual workflow.

Important schema relationships:

* inbox_items has source_id + external_id unique key
* inbox_item_versions belongs to inbox_items
* workflow_versions belongs to workflow_definitions
* workflow_dispatches references inbox_item_id, inbox_item_version_id, workflow_id, workflow_version_id
* workflow_runs references workflow_dispatch_id
* step_runs references workflow_run_id
* artifacts can reference inbox_item_id, workflow_run_id, or step_run_id

## Phase 4: Source interface and manual source

Define the source interface:

```go
type Source interface {
    ID() string
    Type() string
    Sync(ctx context.Context, cursor Cursor) ([]InboxCandidate, Cursor, error)
    Test(ctx context.Context) error
}
```

Define:

```go
type InboxCandidate struct {
    SourceID    string
    ExternalID  string
    EntityType  string
    Title       string
    RawPayload  map[string]any
    Normalized  map[string]any
    ObservedAt  time.Time
}
```

Implement Manual Source first.

Manual source should allow the CLI to create an InboxItem directly:

```bash
runenclave inbox add --source manual --external-id test-1 --title "Test item" --json '{"message":"hello"}'
```

## Phase 5: Inbox service

Implement:

```go
UpsertInboxItem(ctx, InboxCandidate) (InboxItem, InboxItemVersion, changed bool, error)
GetInboxItem(ctx, id)
ListInboxItems(ctx)
ArchiveInboxItem(ctx, id)
IgnoreInboxItem(ctx, id)
```

Deduplication rule:

```txt
source_id + external_id
```

If raw/normalized payload changes, create a new inbox_item_version.

Do not put workflow statuses on InboxItem.

## Phase 6: Workflow YAML MVP

Create initial workflow model.

Example workflow:

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
      result: "Hello from Run Enclave: {{ input.message }}"

sinks:
  - type: markdown
    path: report.md
```

Implement parser and validator.

For MVP validation:

* id required
* name required
* at least one trigger
* at least one step
* step IDs unique
* supported step types only: transform, shell, wait
* supported sink types only: markdown, json, file

Load workflows from:

```txt
~/.config/run-enclave/workflows/
```

Each file load should create or update workflow_definitions and workflow_versions.

A changed YAML should create a new immutable workflow version.

## Phase 7: Trigger evaluator

Implement inbox trigger evaluation.

For each new or updated InboxItemVersion:

* load enabled latest workflow versions
* evaluate triggers
* persist trigger_evaluations
* create workflow_dispatch if matched and policy allows

Policies for MVP:

```txt
once_per_item
once_per_version
manual_only
```

Do not execute workflows inside the trigger evaluator.

## Phase 8: Dispatch queue and run engine

Implement queued dispatch processing:

```txt
queued -> running -> completed/failed/cancelled
```

The dispatch worker should:

1. Pick queued dispatch
2. Mark dispatch running
3. Create WorkflowRun
4. Execute workflow steps sequentially
5. Render sinks
6. Mark run completed or failed
7. Update dispatch status

Run statuses:

```txt
created
running
completed
failed
cancelled
timed_out
waiting_for_approval
waiting_until_time
```

For MVP, only implement:

```txt
created
running
completed
failed
cancelled
```

Add placeholders for waiting statuses.

## Phase 9: Step executor MVP

Create common step contract:

```go
type StepResult struct {
    OK        bool              `json:"ok"`
    Data      map[string]any    `json:"data,omitempty"`
    Artifacts []ArtifactRef     `json:"artifacts,omitempty"`
    Warnings  []string          `json:"warnings,omitempty"`
    Metadata  map[string]any    `json:"metadata,omitempty"`
    Error     *StepError        `json:"error,omitempty"`
}
```

Implement these step types:

### transform

Pure data transformation using simple template rendering.

### shell

Runs a local command with timeout.

For MVP:

* disabled by default unless workflow permissions allow shell
* capture stdout/stderr
* store logs as artifacts
* no secret injection yet

### wait

Accepts duration and pauses execution.

For first implementation, wait can be a no-op or simple timer. Add TODO for durable resume.

## Phase 10: Retry manager

Implement retry policy model but only basic behavior.

```yaml
retry:
  maxAttempts: 2
  backoff: fixed
  delay: 5s
```

Rules:

* Default maxAttempts = 0
* Side-effecting steps must not retry unless explicitly configured
* Persist retry_attempts
* Implement fixed backoff first
* Add exponential as TODO/stub

## Phase 11: Artifact store

Implement local filesystem artifact store.

Default:

```txt
~/.local/share/run-enclave/artifacts/
```

Structure:

```txt
artifacts/
├── inbox/
│   └── inbox_<id>/
│       ├── raw.json
│       └── normalized.json
└── runs/
    └── run_<id>/
        ├── steps/
        │   └── <step_id>/
        │       ├── input.json
        │       ├── output.json
        │       └── log.txt
        └── sinks/
            └── report.md
```

SQLite stores metadata and paths only. Do not store large blobs in SQLite.

## Phase 12: Local API

Implement local HTTP server bound to 127.0.0.1.

Initial endpoints:

```txt
GET  /api/health
GET  /api/inbox
GET  /api/inbox/{id}
POST /api/inbox/{id}/archive
POST /api/inbox/{id}/ignore

GET  /api/workflows
GET  /api/runs
GET  /api/runs/{id}
POST /api/runs/{id}/cancel

GET  /api/sources
POST /api/sources/{id}/test
```

Add local token auth placeholder.

For initial MVP, auth token can be read from:

```txt
~/.config/run-enclave/auth.token
```

If missing, generate it during `runenclave init`.

## Phase 13: CLI

Implement CLI commands that call the local API where possible.

Commands:

```bash
runenclave init
runenclave daemon start
runenclave health
runenclave inbox list
runenclave inbox show <id>
runenclave inbox add --source manual --external-id <id> --title <title> --json <json>
runenclave workflows list
runenclave runs list
runenclave runs show <id>
```

For early bootstrapping, `runenclave init` may directly write files and initialize SQLite.

Prefer:

```txt
CLI -> local daemon API -> services -> SQLite
```

Do not make the CLI mutate SQLite directly except for init/dev commands.

## Phase 14: Daemon supervisor

Implement daemon startup:

1. Load config
2. Ensure directories exist
3. Open SQLite
4. Run migrations
5. Load workflows
6. Start web API
7. Start source sync loop
8. Start trigger evaluation loop
9. Start dispatch worker loop
10. Handle SIGINT/SIGTERM gracefully

MVP worker defaults:

```txt
source workers: 2
dispatch workers: 1
step execution: sequential per run
parallel workflows: allowed
parallel steps within one workflow: not allowed
```

## Phase 15: Minimal end-to-end demo

After implementation, this should work:

```bash
make build
runenclave init
runenclaved
```

In another shell:

```bash
runenclave health
runenclave inbox add --source manual --external-id test-1 --title "First test" --json '{"message":"hello"}'
runenclave inbox list
runenclave workflows list
runenclave runs list
```

Expected result:

* manual InboxItem is created
* manual-hello workflow trigger matches
* workflow_dispatch is created
* workflow_run is created
* transform step runs
* markdown sink is written to artifacts
* run is visible in CLI

## Do not implement yet

Keep these out of the initial setup:

* DAG workflows
* distributed execution
* remote control plane
* multi-user auth
* cloud sync
* hard sandboxing
* plugin marketplace
* Kubernetes
* full web UI
* enterprise policy engine
* full secret broker
* advanced scheduling UI
* GitHub source
* LLM step
* approval UI
* complex RBAC

Add TODOs/interfaces where appropriate, but do not build these yet.

## Quality bar

Every package should have:

* clear interfaces
* minimal unit tests
* no circular dependencies
* context.Context on service methods
* structured errors where useful
* slog logging
* clean shutdown behavior

Add tests for:

* config default path resolution
* workflow YAML parser
* workflow validator
* inbox upsert dedupe
* inbox versioning
* trigger match
* dispatch creation
* transform step execution
* artifact path creation

## Deliverables for this setup task

At the end, produce:

1. Repository scaffold
2. Working build
3. Passing tests
4. Minimal manual workflow demo
5. README with local development instructions
6. docs/architecture.md with the MVP mental model
7. docs/mvp.md with what is intentionally excluded

Do not add unnecessary abstractions. Prefer simple, readable Go code.
