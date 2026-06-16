# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Document `compozy tasks run-multiple` usage in README near the existing `tasks run` and config guidance.
- Add integration coverage proving comma-separated `tasks run-multiple alpha,beta` creates a daemon-owned parent queue whose snapshot contains both requested slugs.
- Cover the `run_multiple_mode = "parallel"` V1 fallback path with observable fallback messaging and enqueued execution evidence.

## Important Decisions
- Keep documentation in README only unless implementation shows generated command docs are also required.
- Reuse the existing in-process daemon CLI test harness rather than adding a separate external process or web smoke test.

## Learnings
- Shared memory says task_04 already centralized comma-separated slug parsing in `internal/core/tasks.ParseCommaSeparatedSlugs`.
- Shared memory says task_05 established queue TUI semantics: queued tabs exist before child runs, completed tabs remain navigable, `Close TUI` detaches, and `Stop Run` cancels the parent queue.
- The existing CLI command help already had `run-multiple` examples; task_06 added a focused help test to keep those examples and shared task-run flags covered.
- In-process CLI stream mode gives deterministic end-to-end evidence for parent queue events and snapshots after both dry-run child workflows complete.
- Focused CLI tests passed, and the full `make verify` gate passed after implementation and tracking updates.

## Files / Surfaces
- `README.md`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/daemon_exec_test_helpers_test.go`
- `internal/cli/root_test.go`
- `.compozy/tasks/multi-task-run/task_06.md`
- `.compozy/tasks/multi-task-run/_tasks.md`

## Errors / Corrections
- `rtk git diff -- ...` treated the path as a revision; use `rtk proxy git diff -- ...` for path-limited diffs.

## Ready for Next Run
- No known follow-up work for task_06.

---

# V2 (Worktree-Backed Parallel) — task_06: Refactor `task_multi` Into a Mode-Aware Scheduler

> NOTE: the V1 notes above are a DIFFERENT task (README docs) under the old numbering. The V2 notes below match the current `_tasks.md` task_06.

## Objective Snapshot
- Turn the sequential-only `task_multi` coordinator into one mode-aware scheduler with explicit `enqueued` and `parallel` branches; preserve enqueued behavior exactly; make the daemon accept `parallel`. No worktree remapping / parallel fanout (task_07/task_08).

## Important Decisions
- Hoisted the shared queue-started + item-queued emission into `runTaskMultiCoordinator`, then dispatched on `prepared.mode`. Extracted `emitTaskMultiQueueStarted`, `emitTaskMultiItemsQueued`, `emitTaskMultiQueueCompleted`; enqueued loop moved verbatim into `runTaskMultiEnqueuedQueue`.
- Parallel branch = guarded scaffold returning sentinel `errTaskMultiParallelNotImplemented`. It cancels queued items via the shared `cancelTaskMultiQueuedItems` (consistent with the existing enqueued failure path, which also emits queue-canceled on a failed parent) so the snapshot is terminal-clean. It starts NO children — running children without worktree isolation is exactly the hazard ADR-004/ADR-005 avoid.
- Added no struct fields (avoids staticcheck write-only `unused`); `prepared.mode` is read in the dispatch switch.

## Learnings
- `internal/daemon/service.go` keys metrics on the daemon run mode `task_multi` (not the enqueued/parallel sub-mode), so it needed no change.
- A nil-`scope` `activeRun` makes `emitTaskMultiEvent` a no-op (guard at top), enabling a fast unit test of the parallel guard (`TestRunManagerRunTaskMultiParallelQueueIsGuarded`, errors.Is the sentinel) without a full run scope.
- Stream rendering: parent terminal error surfaces as `run failed | <error text>` (`internal/cli/run_observe.go:renderObservedRunFailed`); the parallel deferral message therefore appears in stdout.

## Files / Surfaces
- `internal/daemon/task_multi.go` (scheduler dispatch + helpers + sentinel + `resolveTaskMultiMode`)
- `internal/daemon/task_multi_test.go` (mode-accept flip, removed parallel-preflight-reject subtest, added guard + accepted-but-deferred tests)
- `internal/cli/root_command_execution_test.go` (renamed parallel test to accepted-but-deferred)

## Errors / Corrections
- None. fmt/lint(0)/test(3610)/go-build all green on first full run after edits.

## Ready for Next Run
- task_07: replace `runTaskMultiParallelQueue` body with worktree allocation + child remap (allocator from task_05, unwired). task_08: bounded fanout + fail-late aggregation + consume `req.ParallelLimit` + emit `parallel_limit` on the started event, then flip the deferral tests to assert a successful parallel run.
