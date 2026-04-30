# Work Summary: Inbox Task 05

Implements all three items from `docs/tasks/05-inbox.md`.

## What Was Done

### 1. Inbox archive and ignore — CLI subcommands

**File:** `internal/cli/commands.go`

Added `archive <id>` and `ignore <id>` subcommands to `inboxCommand()`. Both POST to the corresponding API endpoints and print the JSON result. The service, repository, and API layers for these operations already existed; only the CLI exposure was missing.

```sh
runloop inbox archive <id>   # POST /api/inbox/{id}/archive → {"ok":true}
runloop inbox ignore <id>    # POST /api/inbox/{id}/ignore  → {"ok":true}
```

No workflow status is stored on `InboxItem`. `archived_at` and `ignored_at` are independent timestamps that do not affect trigger evaluation or dispatch creation.

---

### 2. Inbox versioning tests — `internal/inbox`

**New file:** `internal/inbox/service_test.go` (`package inbox_test`)

Cannot import `internal/store` from `internal/inbox` tests (would create a cycle — `store` imports `inbox`). Tests use a self-contained `fakeRepo` defined in the test file that calls the real `inbox.HashPayload` from `normalize.go`.

**Tests written:**

- `TestServiceDedupesBySameSourceAndExternalID`
  - First upsert with a source+external_id → `changed=true`, `version=1`
  - Second upsert with identical payload → `changed=false`, same item ID, same version ID

- `TestServiceCreatesNewVersionOnPayloadChange`
  - After initial upsert, upsert again with different payload for same source+external_id
  - → `changed=true`, same item ID, `version=2`

---

### 3. `runloop inbox show <id>` — enriched response

Previously `GET /api/inbox/{id}` returned only the bare `InboxItem`.

**Changes:**

- `internal/inbox/service.go` — added `LatestInboxVersion(ctx, itemID)` to the `Repository` interface and `Service`
- `internal/store/repositories.go` — added:
  - `ListDispatchesForItem(ctx, itemID) ([]dispatch.WorkflowDispatch, error)`
  - `GetRunByDispatch(ctx, dispatchID) (runs.WorkflowRun, bool, error)`
- `internal/web/api.go` — `showInbox` handler now returns:

```json
{
  "item": { "id": 1, "sourceId": "manual", "externalId": "t1", ... },
  "version": {
    "id": 1, "inboxItemId": 1, "version": 1,
    "rawPayload": { "message": "hello" },
    "normalized": { "message": "hello" },
    "payloadHash": "..."
  },
  "dispatches": [
    {
      "dispatch": { "id": 1, "status": "completed", ... },
      "run": { "id": 1, "status": "completed", ... }
    }
  ]
}
```

`run` is omitted (`omitempty`) if no workflow run has been created for that dispatch yet.

The CLI `runloop inbox show <id>` required no changes — it already pretty-prints the full API response.

---

## Files Changed

| File | Type | Description |
|------|------|-------------|
| `internal/cli/commands.go` | modified | Add `archive` and `ignore` subcommands |
| `internal/inbox/service.go` | modified | Add `LatestInboxVersion` to interface + service |
| `internal/inbox/service_test.go` | **new** | Versioning contract tests with fake repo |
| `internal/store/repositories.go` | modified | Add `ListDispatchesForItem`, `GetRunByDispatch` |
| `internal/web/api.go` | modified | Enrich `showInbox` response |
| `docs/current_state.md` | modified | Document new inbox routes, CLI commands, test location |

---

## Verification

All pass:

```sh
make build   # ok
make lint    # 0 issues
make test    # ok  runloop/internal/inbox  (TestServiceDedupesBySameSourceAndExternalID, TestServiceCreatesNewVersionOnPayloadChange)
             # ok  runloop/internal/store  (TestInboxUpsertDedupesAndVersionsChangedPayload still passes)
```

Manual smoke test:

```sh
tmp=$(mktemp -d)
HOME="$tmp" ./bin/runloop init
HOME="$tmp" ./bin/runloopd &
HOME="$tmp" ./bin/runloop inbox add --source manual --external-id t1 --title "T1" --json '{"message":"hi"}'
HOME="$tmp" ./bin/runloop inbox show 1       # returns item + version + dispatches[]
HOME="$tmp" ./bin/runloop inbox archive 1    # returns {"ok":true}
HOME="$tmp" ./bin/runloop inbox ignore 1     # returns {"ok":true}
HOME="$tmp" ./bin/runloop inbox list         # archived_at / ignored_at populated
```

---

## Key Design Decisions

- **No workflow status on InboxItem.** `archived_at`/`ignored_at` are inbox-only metadata; dispatch/run status is never stored on the item row.
- **Fake repo in inbox tests, not store import.** `internal/store` imports `internal/inbox`; importing store from inbox tests would be a circular dependency. The fake repo calls the real `HashPayload` so the hash logic is exercised.
- **`showInbox` calls store directly for dispatch/run queries.** The handler already had `a.store`; adding thin service methods for cross-domain queries (dispatches, runs) would be premature abstraction.
