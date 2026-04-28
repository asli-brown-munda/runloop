# Inbox

- [ ] **Inbox archive and ignore** — Implement `ArchiveInboxItem` / `IgnoreInboxItem` end-to-end (service, API endpoints, CLI subcommands) without leaking workflow status onto the inbox row.
- [ ] **Inbox versioning tests** — Cover the dedupe-by-`source_id+external_id` rule and the new-version-on-payload-change rule in `internal/inbox` tests.
- [ ] **`runloop inbox show <id>`** — Pretty-print the latest inbox item version, including raw and normalized payload, plus a list of dispatches/runs derived from it.
