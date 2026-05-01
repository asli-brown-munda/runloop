# Architecture

Runloop is a local-first daemon and CLI for executing AI-powered developer workflows on one machine. The daemon owns source ingestion, inbox normalization, trigger evaluation, dispatching, run execution, step execution, artifact storage, and sink output.

## Flow

```text
Sources -> Inbox -> Trigger Evaluator -> Dispatch Queue -> Workflow Run Engine -> Step Executor -> Sinks
```

## Components

### Sources

Sources discover local or manually submitted items. Filesystem sources use OS file notifications, schedule sources use timed wakeups, GitHub PR sources use timed GraphQL polling, and manual items arrive through the local API. Source state tracks source-specific cursors, external IDs, normalization metadata, and deduplication inputs.

### Inbox

The inbox is the normalized intake layer. Every source item becomes an inbox item with a source ID, external ID, title, entity type, normalized payload, and raw payload where useful.

Inbox/source state is separate from workflow execution state. An inbox item can exist without a run, and a run can fail or retry without mutating the source cursor semantics. This keeps ingestion idempotent and lets execution be rebuilt, retried, or inspected independently.

### Trigger Evaluator

The trigger evaluator compares enabled workflows against inbox items. MVP triggers are inbox-oriented and use simple policies such as `once_per_item` to decide whether an inbox item should become queued work.

### Dispatch Queue

The dispatch queue records work selected by triggers before a workflow run starts. It is the boundary between "this inbox item matched a workflow" and "this workflow execution is running."

### Workflow Run Engine

The workflow run engine creates and advances workflow runs. It owns run status, step status, retry state, timestamps, and references to artifacts. Runs are execution records, not source records.

### Step Executor

The step executor dispatches each step to a handler resolved from a registry rather than a hardcoded switch. Built-in step types are `transform`, `shell`, `wait`, `claude`, and `git_checkout`, each implemented in its own subpackage that self-registers from an `init` function. Steps receive workflow context, inbox context, and previous step output according to the workflow definition. New step types are added by implementing a handler, calling `steps.Register`, and importing the package into the daemon binary; the workflow validator consults the same registry so unsupported types are rejected at load time.

### Sinks

Sinks write final workflow output. The MVP markdown sink writes reports under the run artifact directory.

## Storage

Default local paths:

- Config: `~/.config/runloop/config.yaml`
- Sources: `~/.config/runloop/sources.yaml`
- Workflows: `~/.config/runloop/workflows`
- State database: `~/.local/state/runloop/runloop.db`
- Logs: `~/.local/state/runloop/logs`
- Artifacts: `~/.local/share/runloop/artifacts`

Artifact layout:

```text
~/.local/share/runloop/artifacts/
  inbox/inbox_<id>/
  runs/run_<id>/
    steps/<step-id>/
    sinks/
```

## Local API Boundary

The daemon exposes a local HTTP API on `127.0.0.1:8765` by default. The CLI uses that API for health checks, inbox inspection, manual inbox submission, workflow listing, and run inspection.

This API is local development plumbing for the daemon. It is not a remote control plane.

## Explicit Non-Goals

- DAG workflows
- Distributed execution
- Remote control plane
- Multi-user auth
- Cloud sync
- Hard sandboxing
- Plugin marketplace
- Kubernetes
- Web UI
- Enterprise policy engine
- Full secret broker
- Advanced scheduling UI
- Approval UI
- Complex RBAC
