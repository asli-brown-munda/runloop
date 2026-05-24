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
- Git
- A local POSIX-style shell environment

Build the binaries:

```sh
go mod download
make build
```

Start an isolated development daemon. This keeps Runloop config, state, logs, and artifacts under `.runloop-dev-home` in the repo instead of touching your normal home directory.

Terminal 1:

```sh
export RUNLOOP_DEV_HOME="${RUNLOOP_DEV_HOME:-$PWD/.runloop-dev-home}"
./scripts/dev.sh
```

Terminal 2:

```sh
export RUNLOOP_DEV_HOME="${RUNLOOP_DEV_HOME:-$PWD/.runloop-dev-home}"
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop health
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop workflows list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop sources list
```

Run the sample workflow:

```sh
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop inbox add \
  --source manual \
  --external-id demo-1 \
  --title "Demo item" \
  --json '{"message":"hello"}'

HOME="$RUNLOOP_DEV_HOME" ./bin/runloop inbox list
HOME="$RUNLOOP_DEV_HOME" ./bin/runloop runs list
find "$RUNLOOP_DEV_HOME/.local/share/runloop/artifacts/runs" -path '*/sinks/report.md' -print
```

Expected result:

- `health` returns `{"ok": true}`.
- `workflows list` includes `manual-hello`.
- `sources list` includes `manual`.
- `runs list` shows a completed run after the manual inbox item is added.
- The `find` command prints a `report.md` artifact path.

Stop the daemon with `Ctrl-C`.

For a step-by-step setup guide, including credentials and optional GitHub/Claude configuration, see [Setup Guide](docs/setup.md).

Contributor checks:

```sh
make lint-install
make fmt-check
make test
make lint
make build
```

`make lint` runs local static analysis with `go vet` and `golangci-lint`. The `lint-install` target installs the pinned analyzer version used by this repository.

## Common CLI Commands

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
go run ./cmd/runloop connections list
# after configuring a connection in secrets.yaml:
go run ./cmd/runloop connections test claude.default
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
- Secrets and connections config: `~/.config/runloop/secrets.yaml`
- Local secret files: `~/.config/runloop/secrets/`
- Workflow definitions: `~/.config/runloop/workflows`
- Auth token: `~/.config/runloop/auth.token`
- State directory: `~/.local/state/runloop`
- SQLite database: `~/.local/state/runloop/runloop.db`
- Logs: `~/.local/state/runloop/logs`
- Artifacts: `~/.local/share/runloop/artifacts`
- Inbox artifacts: `~/.local/share/runloop/artifacts/inbox/inbox_<id>`
- Run artifacts: `~/.local/share/runloop/artifacts/runs/run_<id>`

## Documentation

- [Setup Guide](docs/setup.md)
- [Architecture](docs/architecture.md)
- [MVP](docs/mvp.md)
- [Workflow Authoring](docs/workflows.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Local Development Testing](docs/local-development-testing.md)

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
- Approval UI
- Complex RBAC
