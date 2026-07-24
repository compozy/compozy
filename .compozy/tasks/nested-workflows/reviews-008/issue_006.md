---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/daemon/task_multi.go
line: 2245
severity: medium
author: claude-code
provider_ref:
---

# Issue 006: Started child reported as start failure loses run id

## Review Comment

When `emitTaskMultiWorktreeChildStarted` fails, `startTaskMultiWorktreeChild`
returns a fully started child *and* a non-nil error (it cancels the child
asynchronously). `runTaskMultiParallelChild` (lines 1735-1741) treats that as a
start failure and returns before `*runIDOut = child.Run.RunID`, so the child is
absent from `childRunIDs` and from `emitTaskMultiRecoverySummary`.

`settleTaskMultiParallelStartFailure` then emits `ChildFailed` with a hard-coded
empty `childRunID` (lines 1782-1791) and immediately calls
`cleanupSettledTaskWorktree` on a run that is still tearing down.

Failure: the journal write fails on the `child_started` event for item 1 of 3.
Child run `X` exists and is executing, but the parent snapshot shows item 1
`failed` with no `RunID`. No operator or CLI path can find or cancel `X`, its
worktree is deleted underneath it, and the batch summary denominator silently
omits it — so a partially-failed batch can still report as complete.

Fix: propagate `child.Run.RunID` into `runIDOut` and into the failure payload
before settling, and gate the worktree cleanup on the child having actually
settled (see issue 004 — same missing-settle precondition).

## Triage

- Decision: `VALID`
- Root cause: on a `child_started` emit failure, `startTaskMultiWorktreeChild`
  returns a fully populated `taskWorktreeChildRun{Run: childRun, ...}` together with
  a non-nil error (it cancels the child asynchronously). `runTaskMultiParallelChild`
  returned via the start-failure branch *before* writing `*runIDOut`, so the child
  was dropped from `childRunIDs` and from `collectTaskMultiRecoverySummary` (which
  skips empty ids). `settleTaskMultiParallelStartFailure` then emitted `ChildFailed`
  with a hard-coded empty `childRunID`, so no operator/CLI path could find or cancel
  the still-running child, and the summary denominator silently omitted it.
- Fix: `runTaskMultiParallelChild` now records `child.Run.RunID` into `runIDOut`
  before dispatching to any settle path (guarded on a non-empty id, so pure
  allocation failures are unchanged). `settleTaskMultiParallelStartFailure` now
  takes the full `taskWorktreeChildRun`, emits the failure payload with the real
  `child.Run.RunID`, and gates worktree cleanup on the child settling via the shared
  `cleanupSettledTaskMultiChildWorktree` (same missing-settle precondition as issue
  004 — the asynchronously-canceled child is still tearing down).
- Tests: `TestSettleTaskMultiParallelStartFailureRecordsLaunchedChildRunID` in
  `internal/daemon/task_multi_settle_test.go` drives a live parent run and asserts
  the emitted `ChildFailed` payload carries the launched child's run id (not an empty
  string). The settle-gate cleanup is additionally covered by
  `TestCleanupSettledTaskMultiChildWorktree`.
