---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/daemon/task_multi.go
line: 1802
severity: high
author: claude-code
provider_ref:
---

# Issue 004: Wait failure removes the worktree of a still-running child

## Review Comment

`settleTaskMultiParallelWaitFailure` cancels the child and immediately deletes its
worktree:

```go
childCancelErr = m.Cancel(detachContext(active.ctx), child.Run.RunID)
allocation := m.cleanupSettledTaskWorktree(
    context.WithoutCancel(active.ctx),
    prepared.workspace.RootDir,
    child.Allocation,
    taskMultiParallelCleanupPolicy(prepared),
)
```

Neither step waits for the child to settle. `RunManager.Cancel`
(`internal/daemon/run_manager.go:1066-1086`) only calls `active.cancel()` and
returns — it never waits on `active.done`. And
`taskMultiParallelCleanupPolicy` (line 1846) sets
`preserve := prepared.executionKind == apicore.ExecutionKindTaskMultiGroupParallel`,
so for plain `ExecutionKindTaskMultiParallel` preserve is **false** and the
removal really happens.

Failure: `task run-multiple --mode parallel a,b,c`, then cancel the parent a
second later. Children are still starting, their worktrees are clean with
`head == base`, so `inspectTaskWorktreeLifecycle` reports `Removable=true` →
`git worktree remove` deletes the directory out from under a live agent
subprocess, and `DeleteBranchIfAt` drops its result branch. The parent records the
item `canceled` even though the child later settles `completed`.

The same path fires on a transient `GetRun` error from the 100 ms poll ticker
(line 2762), so a single `SQLITE_BUSY` can destroy a healthy child's worktree
while it keeps running.

IT-009 does not cover this: it exercises group-parallel, where `preserve=true`
makes cleanup a no-op — the one mode where the bug is masked.

Fix: before cleanup, wait with a bounded timeout on the child's `active.done` or
its terminal status, and skip removal when the child has not settled. Add a test
in the plain-parallel mode asserting the worktree survives a parent-cancel while
the child is still running.

## Triage

- Decision: `VALID`
- Root cause: `settleTaskMultiParallelWaitFailure` removed the child's worktree
  immediately after `m.Cancel`, which only calls `active.cancel()` and returns
  without waiting (`run_manager.go:1066-1086`). For plain
  `ExecutionKindTaskMultiParallel`, `taskMultiParallelCleanupPolicy` sets
  `preserve=false`, so `cleanupSettledTaskWorktree` really ran `git worktree remove`
  + `DeleteBranchIfAt` on a child that could still be tearing down (parent cancel)
  or entirely healthy (a transient `SQLITE_BUSY` from the 100 ms poll's `GetRun`
  surfaces as a non-context error and lands on this same path). Confirmed the
  poll's `GetRun` error at `waitForTaskMultiChild` returns a wrapped, non-context
  error, so a single DB hiccup could destroy a live child's worktree.
- Fix: added `waitForSettledTaskMultiChild`, a bounded, context-detached wait for
  the child to reach a terminal status (or close its `done` channel), and
  `cleanupSettledTaskMultiChildWorktree`, which only delegates to
  `cleanupSettledTaskWorktree` once the child has actually settled. A child that
  has not settled within `taskMultiChildSettleTimeout` keeps its worktree
  (`WorktreeStatus=preserved`) instead of being removed out from under a live
  subprocess. Both the wait-failure and start-failure paths now route through this
  gate. The normal event-driven case returns as soon as the child closes `done`,
  well inside the bound.
- Tests: `TestWaitForSettledTaskMultiChild` (empty id, already-terminal,
  still-running-not-settled, done-channel) and
  `TestCleanupSettledTaskMultiChildWorktree` (plain-parallel worktree preserved
  while the child is still running; unallocated child left untouched) in
  `internal/daemon/task_multi_settle_test.go`.
