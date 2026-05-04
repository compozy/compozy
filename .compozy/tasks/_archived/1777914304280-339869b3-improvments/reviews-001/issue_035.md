---
status: resolved
file: internal/daemon/shutdown.go
line: 214
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:ff619dee4bb5
review_hash: ff619dee4bb5
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 035: Wrap joined shutdown errors with operation context.
## Review Comment

`errors.Join` is good here, but both branches currently return raw close errors. Wrap each close failure first so logs/errors clearly identify which shutdown step failed.

As per coding guidelines, Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`.

Also applies to: 249-257

## Triage

- Decision: `valid`
- Notes: Confirmed shutdown joined raw cache/event-bus close errors, obscuring which close step failed. Wrapped each close error with operation-specific shutdown context before joining.
