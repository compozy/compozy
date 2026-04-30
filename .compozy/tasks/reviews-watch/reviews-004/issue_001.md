---
provider: coderabbit
pr: "133"
round: 4
round_created_at: 2026-04-30T22:06:27.568795Z
status: resolved
file: internal/core/sync_test.go
line: 165
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208822110,nitpick_hash:71bed183821c
review_hash: 71bed183821c
source_review_id: "4208822110"
source_review_submitted_at: "2026-04-30T22:05:49Z"
---

# Issue 001: Also assert review_issues cleanup in the pruning test.
## Review Comment

This test already verifies `workflows`, `task_items`, and `review_rounds` deletion. Adding a `review_issues` count assertion would close the loop on orphaned review data after pruning.

## Triage

- Decision: `valid`
- Notes:
  - The pruning test deletes a workflow after seeding one review issue, but it only asserts `review_rounds` cleanup. That leaves the `review_issues` cascade path unverified.
  - Root cause: the new regression test stops one level too early in the persistence graph, so orphaned review issue rows would not be caught by this case.
  - Fix approach: capture the pruned round id after the first sync and assert `review_issues` rows for that round are zero after pruning.
  - Verification: `go test ./internal/core` passed during focused validation, and `make verify` passed after the final patch.
