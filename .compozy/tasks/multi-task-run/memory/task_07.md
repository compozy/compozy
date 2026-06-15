# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

> NOTE: there were no V1 notes for task_07 (the old V1 breakdown ended at task_06).
> The notes below are the V2 (worktree-backed parallel) task_07 from the current `_tasks.md`.

---

# V2 (Worktree-Backed Parallel) — task_07: Register and Remap Parallel Children to Worktree Workspaces

## Objective Snapshot
- Wire the task_05 worktree allocator into the scheduler and replace the guarded `runTaskMultiParallelQueue` scaffold so each parallel child runs in an isolated detached worktree: register the worktree as its own workspace row, remap child `RuntimeConfig.{WorkspaceRoot,TasksDir,ParentRunID}`, emit worktree metadata before launch.
- Execution boundary: bounded concurrent fanout + fail-late aggregation + flipping deferral tests to assert a successful PARALLEL run = task_08. task_07 starts children one-at-a-time (interim, fail-fast) reusing the shared per-child machinery.

## Important Decisions
- Threaded `WorktreesRoot string` into `RunManagerConfig`; `NewRunManager` builds `worktreeAllocator *taskMultiWorktreeAllocator` via `newTaskMultiWorktreeAllocator(cfg.WorktreesRoot)`. `host.go` passes `currentHost.Paths().WorktreesDir`; the field is READ in production (`resolveTaskMultiParallelBase` + `startTaskMultiWorktreeChild`) so staticcheck `unused` is satisfied.
- `runTaskMultiParallelQueue`: `ResolveBase` once (detached HEAD / non-git surfaces as the parent failure, cancels queued items), then sequential loop calling `runTaskMultiWorktreeChildAt`. Refactored `runTaskMultiChildAt` into shared `handleTaskMultiChildStartFailure` + `awaitTaskMultiChild` so enqueued and parallel reuse identical terminal/cancel handling (enqueued behavior unchanged).
- `startTaskMultiWorktreeChild`: Allocate -> emit `item_queued` WITH worktree fields (before launch) -> `resolveWorkflowContext(allocation.Path, slug)` to register/resolve the worktree workspace row + workflow identity -> remap onto `workspaceRow.RootDir` (canonical, ADR-007 alignment) -> `startRun` with worktree workspace + ParentRunID -> emit `child_started` with worktree path.
- Removed sentinel `errTaskMultiParallelNotImplemented` (now dead). Pure helpers: `remapTaskMultiChildRuntime` (clone + set WorkspaceRoot/TasksDir/ParentRunID, preserve other overrides), `requireTaskMultiWorktreeTaskDir` (slug-specific missing-dir error), `taskMultiWorktreeItemPayload` (adds worktree fields to the item payload).

## Learnings
- Remap target is the REGISTERED worktree workspace row `RootDir` (EvalSymlinks-canonical), not the raw `allocation.Path`, so DB workspace identity == runtime `WorkspaceRoot` (ADR-007). The event `WorktreePath` keeps the allocator's raw `allocation.Path`; on macOS these differ only by the `/private` symlink prefix (same dir).
- `coreworkspace.Discover` returns the worktree itself because the detached worktree contains a committed `.compozy/` tree; the `~/.compozy` global marker is explicitly skipped, so worktrees under the home dir do not resolve to the home workspace.
- Integration test needs a REAL git workspace: git-init `env.workspaceRoot` + commit the `.compozy/tasks/<slug>` files so the detached worktree contains them (`commitTaskMultiGitWorkspace` helper; `requireGitForTaskMulti` skips when git is absent). `newRunManagerTestEnv` now sets `WorktreesRoot: paths.WorktreesDir`.
- Capturing the child `RuntimeConfig` in the `execute` mock is the cleanest proof the remap reached execution (WorkspaceRoot/TasksDir/ParentRunID).

## Files / Surfaces
- `internal/daemon/run_manager.go` (`RunManagerConfig.WorktreesRoot`, `RunManager.worktreeAllocator`, constructed in `NewRunManager`)
- `internal/daemon/host.go` (pass `WorktreesDir`)
- `internal/daemon/task_multi.go` (parallel queue, base resolution, worktree child start, remap/dir/payload helpers, shared start-failure/await helpers, sentinel removed)
- `internal/daemon/task_multi_test.go` (remap + dir unit tests, base-before-children test, real-git integration test, helpers)
- `internal/daemon/run_manager_test.go` (`WorktreesRoot` in test env)
- `internal/cli/root_command_execution_test.go` (parallel deferral test -> requires-git-workspace test)

## Errors / Corrections
- None. fmt / lint (0 issues) / test (3618, 3 intentional skips) / go-build all green.

## Ready for Next Run
- task_08 (bounded fanout + fail-late): replace the sequential loop in `runTaskMultiParallelQueue` with a context-aware semaphore bounded by the resolved limit + goroutines owned by the parent (WaitGroup); change fail-fast (`awaitTaskMultiChild` cancels queued siblings) to FAIL-LATE (siblings continue; aggregate failed-slug summary). Consume `req.ParallelLimit` (missing/0 -> configured/default per task_03 decision) and emit `parallel_limit` on the `started` event. Reuse `startTaskMultiWorktreeChild` (per-child registration/remap) unchanged. Then flip `TestRunManagerTaskRunMultipleParallelRegistersChildrenUnderWorktreeWorkspaces` style tests / CLI test to assert a successful concurrent parallel run (with a git workspace + WorktreesRoot bootstrap).
