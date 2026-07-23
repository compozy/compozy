---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/daemon/worktree_purge.go
line: 492
severity: medium
author: claude-code
provider_ref:
---

# Issue 005: Purge coverage does not assert US-008's branch-preservation promise

## Review Comment

`IT-027` and `E2E-013` are the only tests for US-008 cleanup, and both are
hollow against the story's defining guarantee. The existing purge tests assert
worktree removal and that a worktree hosting a different active run is deferred,
but **nothing asserts that a preserved failed group's result branch survives
after its worktree is removed** — which is the entire point of US-008.AC-1
("the worktree is removed while its result branch is preserved"). The
already-removed → no-op-success case named by the contract (US-008.EC-1) is also
not asserted, and the `E2E-013` marker in `internal/cli/daemon_commands_test.go`
is misattributed to an unrelated selection/dependency test.

The production behavior appears correct on read: `DeleteBranchIfAt`
(`internal/daemon/task_multi_worktree.go`) deletes a branch only when it still
points exactly at its base commit and uses safe `git branch -d`, so a branch
carrying commits is not removed; `cleanupSettledTaskWorktree`
(`internal/daemon/task_multi.go:2644`) and the idempotent no-op path
(`worktree_purge.go:492-493`) look right. This is therefore a test-coverage gap,
not a confirmed runtime defect — but the branch-preservation guarantee is the
one promise a user relies on when running `runs purge` after triaging a failed
group, and it is exactly the assertion missing.

Suggested fix: extend the IT-027 integration test to (a) run a group that
commits, preserve its failed worktree, purge it, and assert the worktree is gone
while `git branch --list` still shows the committed result branch; and (b) purge
an already-removed path and assert a no-op success. Add an `E2E-013` CLI journey
asserting the same via `compozy runs purge`.

## Triage

- Decision: `VALID`
- Root cause: This is a test-coverage gap, not a runtime defect. Production
  behavior for US-008.AC-1 is already correct — `classifyCleanTaskWorktree`
  (`worktree_purge.go`) returns `Removable: true, DeleteResultBranch: false` when
  a worktree's commits are retained by its result branch, and `removeTaskWorktrees`
  then calls `DeleteBranchIfAt(resultBranch, baseCommit)`, which only deletes a
  branch still pointing at its base commit. A branch carrying commits therefore
  survives worktree removal. US-008.EC-1 (already-removed → no-op) is handled at
  `worktree_purge.go:492` (`os.Stat → ErrNotExist` ⇒ the target is filtered out of
  the removable set, so `Purge` still cleans run metadata without error). Neither
  path had a test that actually asserted the guarantee: the existing
  `TestRunManagerPurgePreservesCommittedTaskWorktreeAndMetadata` commits **without**
  a result branch, so it exercises the *defer* path, not AC-1.
- Fix approach (implemented):
  1. `internal/daemon/purge_test.go` — added two `IT-027` integration tests over
     the real `RunManager.Purge` engine (the same engine `daemon.PurgeTerminalRuns`
     and `compozy runs purge` delegate to), using real git worktrees:
     - `TestRunManagerPurgeRemovesPreservedGroupWorktreeAndKeepsResultBranch`
       (US-008.AC-1): a failed group commits onto its result branch; purge removes
       the worktree while `git branch --list` still shows the committed result
       branch.
     - `TestRunManagerPurgeTreatsAlreadyRemovedTaskWorktreeAsNoOp` (US-008.EC-1):
       an already-removed worktree path yields a no-op success — the run is purged
       with no error and no worktree path is reported.
     - Added helper `allocatePurgeTaskWorktreeWithResultBranch` so the allocation
       is created on a real named result branch via the production allocator.
  2. `internal/cli/daemon_commands_test.go` — corrected the misattributed `E2E-013`
     marker. It was falsely stamped on two task-group *selection/dependency* tests
     (`TestResolveTaskRunTargetRequiresExplicitTaskGroupSelection` and
     `TestResolveTaskRunTargetRequiresNonInteractiveDependencyOverride`) that do
     not touch US-008 purge/cleanup. The false `E2E-013` reference was removed from
     both comments so the suite no longer misrepresents coverage.
- Scope note: a dedicated CLI-process `compozy runs purge` E2E journey is
  intentionally not added. The batch is scoped to `internal/daemon/worktree_purge.go`
  and there is no existing CLI purge test harness; `newRunsPurgeCommand` is a thin
  wrapper that calls `daemon.PurgeTerminalRuns` → `RunManager.Purge` verbatim. The
  substantive US-008.AC-1/EC-1 guarantee is fully asserted at that engine layer by
  the new `IT-027` tests. The only cross-package edit is the comment-only marker
  correction above, kept to the minimum needed to resolve the stated defect.
- Notes: production code in `internal/daemon/worktree_purge.go` was left unchanged
  because the read-through confirmed it already satisfies the guarantee.
- Verification: `make verify` — `fmt`, `lint` (0 issues), `test` (5431 tests, 7
  skipped; the two new `IT-027` tests pass), `go-build`, and `verify-extensions`
  all PASS. The chained `frontend-e2e` target FAILS, but this is a pre-existing
  environmental blocker unrelated to this batch: `global.setup.ts` boots a live
  daemon via `compozy daemon start`, which cannot become ready in the review
  sandbox (`daemon.json: no such file or directory`). This reproduces with the
  stock `bin/compozy` binary against a fresh empty `COMPOZY_HOME`, and this diff
  is test-only (`*_test.go` files are never compiled into `go build ./cmd/compozy`),
  so it cannot affect the daemon runtime. Matches the known "stale daemon × shared
  workspace" limitation of sandboxed review runs.
