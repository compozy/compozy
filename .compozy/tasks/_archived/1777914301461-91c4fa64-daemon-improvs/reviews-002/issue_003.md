---
status: resolved
file: internal/daemon/boot_integration_test.go
line: 533
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148862187,nitpick_hash:1856fa3962c8
review_hash: 1856fa3962c8
source_review_id: "4148862187"
source_review_submitted_at: "2026-04-21T15:19:50Z"
---

# Issue 003: Replace the time.Sleep polling loops with ticker-driven waits.
## Review Comment

These helpers busy-poll with `time.Sleep(50 * time.Millisecond)`. That makes the integration tests slower/flakier and is out of line with the other wait helpers in this file, which already use `Ticker`/`Timer` based synchronization.

As per coding guidelines, "NEVER use `time.Sleep()` in orchestration — use proper synchronization primitives instead".

## Triage

- Decision: `valid`
- Root cause: `waitForLogContains` and `waitForStderrContains` use blind `time.Sleep(50 * time.Millisecond)` polling loops, which is inconsistent with the ticker/timer helpers elsewhere in the file and can make the integration waits slower and less predictable.
- Plan: rewrite both helpers to use `time.Ticker` plus `time.Timer` based waiting while preserving the current timeout behavior and failure messages.
- Resolution: rewrote both wait helpers in `internal/daemon/boot_integration_test.go` to poll with `time.Ticker` and `time.Timer`, preserving the same timeout windows and failure output without blind sleeps.
- Regression coverage: the managed-daemon stop and logging-mode integration tests still rely on these helpers, so the focused daemon package run covers the updated wait behavior directly.
- Verification: `go test ./internal/daemon -run 'Test(ManagedDaemonStopEndpointShutsDownAndRemovesSocket|ManagedDaemonRunModesControlLogging)$' -count=1` passed. `make verify` then passed with `2544` tests and `2` skipped helper-process tests.
