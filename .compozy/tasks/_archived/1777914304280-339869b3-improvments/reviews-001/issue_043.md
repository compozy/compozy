---
status: resolved
file: internal/store/globaldb/sync_test.go
line: 421
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:459221b99748
review_hash: 459221b99748
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 043: Guard the slice length before reading snapshots[0].
## Review Comment

If `ListArtifactSnapshots()` returns zero rows, this failure path panics while formatting the assertion message and hides the real problem.

## Triage

- Decision: `VALID`
- Notes: The assertion read `snapshots[0]` in the failure message even when `ListArtifactSnapshots` returned an empty slice. The fix asserts the length first, then reads `snapshots[0]` only after the slice is known to contain one row.
