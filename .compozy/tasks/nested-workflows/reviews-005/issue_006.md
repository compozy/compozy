---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: pending
file: internal/daemon/task_multi_summary.go
line: 123
severity: medium
author: claude-code
provider_ref:
---

# Issue 006: Recovery summary can delay finalization by N times five seconds

## Review Comment

`collectTaskMultiRecoverySummary` reads child evidence serially, while each `readChildSummaryEvidence` call creates a fresh five-second timeout. Because summary collection runs before the parent terminal result is emitted, N unavailable or wedged child stores can delay an already-settled parent by `N * 5s`. This contradicts the stated best-effort invariant that a wedged child store must not hold finalization open and becomes severe for large multi-run queues.

Use one batch-level deadline or bounded parallel fan-out with owned cancellation so total summary latency has one fixed upper bound. Add a test with several blocked child stores and assert one aggregate deadline rather than one timeout per child.

## Triage

- Decision: `UNREVIEWED`
- Notes:
