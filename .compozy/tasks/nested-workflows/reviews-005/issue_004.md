---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: resolved
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

- Decision: `VALID`
- Notes: `latestReviewRoundPickerSummary` resolves only the newest non-empty review directory and passes that single round to `readReviewRoundPickerSummary`. This differs from the lifecycle completion gate, which discovers and scans every review round. Therefore, an unresolved issue in an older round is omitted when a newer round is fully resolved, causing the picker to label and lock an actionable target as having no pending issues. The fix will preserve the newest non-empty round for display while aggregating issue and pending counts across every non-empty round. Regression tests will cover both ordinary workflows and initiative Task Groups with an older pending round and a newer resolved round.
- Verification: The generated review-worktree path exceeds macOS's Unix-socket path limit, so its direct frontend E2E daemon failed to bind. The final full `make verify` ran from a short-path, byte-identical disposable copy and passed with zero warnings/errors: 5,299 Go tests, 7 Playwright tests, lint, build, and extension verification. One pre-existing subprocess timing test failed on the first copied run, passed in isolation, and passed in the final full run.
