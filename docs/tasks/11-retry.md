# Retry

- [ ] **Retry policy parsing** — Parse the `retry` block on a step (maxAttempts, backoff, delay) in the workflow model and validator.
- [ ] **Fixed backoff retries** — Implement `internal/retry/policy.go` and `backoff.go` for fixed delay retries, persisted in `retry_attempts`.
- [ ] **No-retry default for side-effecting steps** — Default `maxAttempts` to 0 and require an explicit opt-in for `shell` and other side-effecting steps.
