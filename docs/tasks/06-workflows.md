# Workflows

- [x] **Workflow file watcher** — Reload changed YAML files in `~/.config/runloop/workflows/` while the daemon is running, creating a new `workflow_versions` row only on real content change.
- [x] **Disable / enable workflow** — Add `runloop workflows disable <id>` and `enable <id>` plus matching API endpoints, backed by the `enabled` flag on `workflow_definitions`.
- [x] **Validator coverage** — Extend `internal/workflows/validator.go` to reject duplicate step IDs, unsupported step types, unsupported sink types, and unknown trigger fields with line-level context.
- [x] **Workflow show command** — `runloop workflows show <id>` should print the latest version YAML, current enabled state, and the most recent dispatches.
