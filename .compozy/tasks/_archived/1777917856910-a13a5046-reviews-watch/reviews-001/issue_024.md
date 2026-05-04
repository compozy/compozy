---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/store/globaldb/sync.go
line: 228
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:59aff765c5b9
review_hash: 59aff765c5b9
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 024: Potential redundant active run check after atomic conditional delete.
## Review Comment

Lines 287-295 re-count active runs after `deleteActiveWorkflowIfNoActiveRuns` returns `deleted=false`. However, the DELETE at lines 319-327 is atomic with a `NOT EXISTS` subquery that already checks for active runs. If the delete didn't happen, the `archiveReasonActiveRuns` reason is always applied regardless of the actual cause (which could also be `archived_at IS NOT NULL`).

This may report misleading skip reasons for already-archived workflows.

## Triage

- Decision: `valid`
- Notes: After the conditional delete fails, the code always records `archiveReasonActiveRuns` even if a concurrent archive removed the row from the active set and the active-run recount returns zero. I will treat the zero-active-runs case as a concurrent state change instead of reporting a false skip reason, and I will cover that behavior with a focused sync test.
- Resolution: Added a production helper that only records active-run skips when active runs actually remain, plus focused sync coverage; the full verification pipeline passed afterwards.
