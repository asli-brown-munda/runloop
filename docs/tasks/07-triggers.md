# Triggers

- [ ] **Trigger evaluation persistence** — Persist every evaluation (matched or not) to `trigger_evaluations` with the inbox item version and workflow version it ran against.
- [ ] **Expression-based matching (optional)** — Spike `expr-lang/expr` for trigger conditions behind a feature flag; keep the current direct policy matcher as the default.
- [ ] **`once_per_version` semantics test** — Lock down the difference between `once_per_item` and `once_per_version` with table-driven tests in `internal/triggers/`.
