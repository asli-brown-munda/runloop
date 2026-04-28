# Runloop

Runloop is a Go local-first workflow automation runtime for AI-powered developer work. It watches local sources, normalizes items into an inbox, evaluates triggers, queues matching work, and executes workflow steps on the same machine.

The MVP mental model is:

```text
Sources -> Inbox -> Trigger Evaluator -> Dispatch Queue -> Workflow Run Engine -> Step Executor -> Sinks
```

Inbox/source state is separate from workflow execution state. The inbox records what local sources have produced and what has been normalized. Workflow execution state records queued dispatches, runs, steps, retries, artifacts, and sink output.

## Local Development

Prerequisites:

- Go 1.24 or newer
- A local shell environment

Common commands:

```sh
go mod download
make lint-install
make test
make lint
make build
```

`make lint` runs local static analysis with `go vet` and `golangci-lint`. The `lint-install` target installs the pinned analyzer version used by this repository.

Initialize local config and examples:

```sh
go run ./cmd/runloop init
```

Start the daemon in the foreground:

```sh
go run ./cmd/runloopd
```

In another terminal, use the CLI:

```sh
go run ./cmd/runloop health
go run ./cmd/runloop workflows list
go run ./cmd/runloop inbox add --external-id demo-1 --title "Demo item" --json '{"message":"hello"}'
go run ./cmd/runloop inbox list
go run ./cmd/runloop runs list
```

The daemon listens on `127.0.0.1:8765` by default.

## Default Paths

Runloop follows local user paths:

- Config directory: `~/.config/runloop`
- Main config: `~/.config/runloop/config.yaml`
- Sources config: `~/.config/runloop/sources.yaml`
- Workflow definitions: `~/.config/runloop/workflows`
- Auth token: `~/.config/runloop/auth.token`
- State directory: `~/.local/state/runloop`
- SQLite database: `~/.local/state/runloop/runloop.db`
- Logs: `~/.local/state/runloop/logs`
- Artifacts: `~/.local/share/runloop/artifacts`
- Inbox artifacts: `~/.local/share/runloop/artifacts/inbox/inbox_<id>`
- Run artifacts: `~/.local/share/runloop/artifacts/runs/run_<id>`

## Documentation

- [Architecture](docs/architecture.md)
- [MVP](docs/mvp.md)

## Excluded From This Milestone

Runloop is intentionally not building these features in the MVP documentation milestone:

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
- GitHub source
- LLM step
- Approval UI
- Complex RBAC
