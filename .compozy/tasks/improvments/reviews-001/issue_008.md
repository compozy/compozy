---
status: resolved
file: internal/api/core/handlers_test.go
line: 215
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:99090e94f3e9
review_hash: 99090e94f3e9
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 008: Give the workspace-stream feeder goroutine a shutdown path.
## Review Comment

If the test fails before the heartbeat is observed, this goroutine blocks forever on `sendOverflow`. Thread it through a cancellable context and/or a `sync.WaitGroup` so cleanup can stop it deterministically. As per coding guidelines, "Every goroutine must have explicit ownership and shutdown via `context.Context` cancellation" and "No fire-and-forget goroutines; track all goroutines with `sync.WaitGroup` or equivalent".

## Triage

- Decision: `VALID`
- Notes: The workspace socket feeder goroutine could block forever if the test failed before the overflow signal. Added a cancellable context and `sync.WaitGroup`, selected sends on cancellation, and joined the goroutine in cleanup.
