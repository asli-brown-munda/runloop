# Triggers

- [x] **Trigger evaluation persistence** — Persist every evaluation (matched or not) to `trigger_evaluations` with the inbox item version and workflow version it ran against.
- [ ] **Expression-based matching (deferred optional)** — Spike `expr-lang/expr` for trigger conditions behind a feature flag only if workflow authoring needs richer matching. Keep the current direct source/entity/policy matcher as the default MVP behavior.
- [x] **`once_per_version` semantics test** — Lock down the difference between `once_per_item` and `once_per_version` with table-driven tests in `internal/triggers/`.
