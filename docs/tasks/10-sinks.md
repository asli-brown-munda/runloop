# Sinks

- [x] **JSON sink** — Implement a `json` sink that writes the final run context as `report.json` under `runs/run_<id>/sinks/`.
- [x] **File sink** — Implement a generic `file` sink that writes a templated body to a configured filename, validated against path traversal.
- [x] **Sink output records** — Persist sink writes in `sink_outputs` so the API can list them per run.
