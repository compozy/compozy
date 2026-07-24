---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/cli/task_group_picker.go
line: 419
severity: medium
author: claude-code
provider_ref:
---

# Issue 010: Picker advertises pending issues the fix run cannot reach

## Review Comment

`reviewRoundPickerSummaryAcrossRounds` sums `PendingIssueCount` over *all* rounds
while overwriting `summary.Round` with each round that has any issues (rounds
ascending), so `Round` ends up as the highest round containing issues — not the
highest round containing *pending* issues.

Dispatch works on a single round: `resolveReviewRound`
(`internal/cli/reviews_exec_daemon.go:1630`) picks the newest round directory
containing `issue_*.md`, and `stopReviewFixWithoutPendingIssues` (line 439)
inspects only that round.

Failure: `reviews-001` has 2 unresolved issues; `reviews-002` has issues that are
all resolved. The picker renders "Review round 2 — 2 issues pending" and the row
is selectable, because `reviewFixSelectionBlockedReason` sees pending=2. The run
then resolves round 2, finds 0 pending, prints "No pending review issues for X in
round 002." and exits 0 having done nothing. The advertised issues are
unreachable unless the user guesses `--round 1`. The label is wrong too: round 2
has zero pending.

Lineage: prior round 5 issue 004 ("Review picker hides pending issues from older
rounds") introduced the across-rounds sum. That fixed the display half; the
dispatch half was never updated to match, which is what this issue reports.

Fix: have the summary carry the newest round that still has pending issues and
make `resolveReviewRound` prefer it, or scope the picker summary to the round that
will actually be dispatched. Add a test for the pending-in-older-round shape
asserting the picker label and the dispatched round agree.

## Triage

- Decision: `VALID`
- Root cause: `reviewRoundPickerSummaryAcrossRounds` (picker, `task_group_picker.go:402`)
  iterated rounds ascending and set `summary.Round = round` for every round with
  any issues while summing `PendingIssueCount` across all rounds. `Round` therefore
  ended on the highest round containing *any* issues, not the highest round with
  *pending* issues, and the pending count was a cross-round total. Meanwhile the
  daemon dispatch (`resolveReviewRound` → `latestLocalReviewRoundForTarget` →
  `latestLocalReviewRoundInDir`, `reviews_exec_daemon.go`) selected the newest
  round directory that merely *contains* an `issue_*.md`, ignoring resolved/pending
  status. With `reviews-001` pending and `reviews-002` fully resolved the picker
  rendered "Review round 2 — 2 issues pending" and left the row selectable, yet
  dispatch resolved round 2, saw 0 pending via `stopReviewFixWithoutPendingIssues`,
  and exited 0. The advertised issues were unreachable without `--round 1`.
- Fix approach (reviewer option 1 — make label and dispatch agree on the same
  round): introduce a shared `reviewDispatchRoundSummary(reviewRoot)` that scans
  rounds newest-first and returns the newest round with pending issues (falling
  back to the newest round with any issues once all are resolved), scoping the
  summary's `Round`/`PendingIssueCount`/`IssueCount` to that single round. The
  picker uses it for the label + selectability, and the daemon's
  `latestLocalReviewRoundInDir` resolves the round through the same function, so
  the row a user selects and the round the run targets are always identical. The
  now-dead `parseReviewRoundDirName`/`reviewRoundHasIssueFile` helpers are removed.
- Out-of-scope note: the fix requires the daemon's round resolution to match, so
  `internal/cli/reviews_exec_daemon.go` is edited minimally (delegate round
  selection to the shared helper + drop the two unused helpers). Two daemon
  round-resolution tests that used status-less placeholder `# issue` files are
  updated to write valid pending front matter, because round selection now parses
  issue status (as `stopReviewFixWithoutPendingIssues` already did downstream).
- Tests: updated `TestReviewFixPickerShowsOlderPendingIssue*` to assert the
  reachable round label (round 1, not the buggy round 2), and added
  `TestReviewFixPickerLabelAgreesWithDispatchedRound` asserting picker label and
  `latestLocalReviewRoundInDir` resolve to the same round for the
  pending-in-older-round shape.
