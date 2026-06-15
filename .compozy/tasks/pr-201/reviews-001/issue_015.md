---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/multi_remote.go
line: 815
severity: major
author: coderabbitai[bot]
provider_ref: review:4499860579,nitpick_hash:0bdee73215d0
review_hash: 0bdee73215d0
source_review_id: "4499860579"
source_review_submitted_at: "2026-06-15T18:04:32Z"
---

# Issue 015: Wire job-control callbacks for event-created child models.
## Review Comment

When child events arrive before or without a bootstrap snapshot, the placeholder child never gets `onJobControl`, so pause/message shortcuts in that tab cannot call the daemon even though the callbacks exist on the multi-run model.

<!-- cr-comment:v1:0d2b239d7b4210b201457468 -->

## Triage

- Decision: `valid`
- Root cause: `handleChildEvent` creates placeholder child models without wiring `onJobControl`, so pause/message requests in an event-created child tab cannot reach the daemon callbacks.
- Fix approach: pass the multi-run model's pause/message callbacks through placeholder creation and wire the same remote job-control handler used by bootstrap snapshots. Regression coverage belongs in `internal/core/run/ui/multi_remote_test.go`, the canonical suite for this behavior.

## Resolution

- Resolved by wiring job-control callbacks into placeholder child models.
- Verification: `rtk make verify` exited 0 after the code changes.
