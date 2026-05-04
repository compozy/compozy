---
status: resolved
file: internal/daemon/boot_integration_test.go
line: 514
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:7dd7eebf4694
review_hash: 7dd7eebf4694
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 017: Consider using a ticker instead of time.Sleep for consistency.
## Review Comment

The other polling helpers (`waitForHealthyDaemon`, `waitForManagedDaemonReady`) use `time.NewTicker` with proper cleanup, but this helper uses `time.Sleep`. While acceptable in test code, using a consistent pattern would improve readability.

## Triage

- Decision: `valid`
- Root cause: `waitForDaemonState` polls with a raw `time.Sleep` loop while adjacent helpers in the same file use managed tickers. This helper should follow the same owned-ticker pattern for consistency and clearer lifecycle handling.
- Fix approach: replace the sleep loop with a ticker-based poll loop.
- Resolution: `waitForDaemonState` now uses a managed ticker and timeout timer instead of a raw sleep loop.
- Regression coverage: `go test ./internal/cli ./internal/core/run/journal ./internal/daemon` passed after the helper update.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
