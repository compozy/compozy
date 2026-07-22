---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: pending
file: internal/cli/task_group_picker.go
line: 344
severity: high
author: claude-code
provider_ref:
---

# Issue 004: Review picker hides pending issues from older rounds

## Review Comment

The review-fix picker summarizes only the newest non-empty round. If `reviews-001` still contains a pending issue while `reviews-002` is resolved, the picker reports "No issues pending" and locks the target at lines 320-329. The completion gate correctly scans every round, so it then refuses lifecycle completion, while the normal interactive fix flow offers no selectable target for the older issue.

Aggregate pending issue state across all rounds, or make the round an explicit picker dimension and honor the selected/`--round` value. The UI must never claim there are no pending issues while any earlier round remains unresolved. Add ordinary-workflow and Task Group tests with an older pending round and a newer resolved round.

## Triage

- Decision: `UNREVIEWED`
- Notes:
