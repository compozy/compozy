# Task 08 Memory

## V2: Bounded Parallel Fanout and Fail-Late Aggregation

Implemented true concurrent parallel multi-run in `internal/daemon/task_multi.go`,
replacing task_07's sequential/fail-fast interim loop.

### What changed
- `runTaskMultiParallelQueue`: counting semaphore (`chan struct{}` sized to the
  resolved limit) bounds concurrent child starts; every worker goroutine is
  parent-owned and joined via `sync.WaitGroup` before the parent terminal status.
  The scheduling loop breaks on `context.Cause(active.ctx)` (cancellation) and
  selects on `active.ctx.Done()` while acquiring a slot.
- `runTaskMultiParallelChild`: fail-late child runner. Reuses
  `startTaskMultiWorktreeChild` (task_07) + `waitForTaskMultiChild` +
  `finishTaskMultiChild`, but NEVER cancels siblings on a child failure/start
  error (the enqueued `awaitTaskMultiChild` still cancels siblings — unchanged).
- `finalizeTaskMultiParallel`: on cancellation, marks not-started items canceled
  via `cancelTaskMultiQueuedItems(launched, total)` and returns the cause;
  otherwise returns the aggregate failure or emits `queue_completed`.
- `aggregateTaskMultiParallelResult` (pure): nil on all-success; else one error
  naming failed slugs in queue order, `errors.Join`-wrapping child errors.
- Limit resolution: `preparedTaskMulti.parallelLimit` resolved once in
  `prepareTaskMultiStart` (parallel mode only) via `parallelLimitForRequest`
  (`req.ParallelLimit>0` wins, else `loadProjectConfig().Tasks.Run.EffectiveRunMultipleParallelLimit()`),
  using pure `resolveTaskMultiParallelLimit(reqLimit, configuredLimit)` clamped ≥1.
  Emitted as `parallel_limit` on the `started` event (observability only; not in
  snapshot items).
- `activeRun.emitMu` (new `sync.Mutex`): serializes `emitTaskMultiEvent` so
  concurrent child workers append item events atomically and in per-item order.
- Removed unused `runTaskMultiWorktreeChildAt`; sentinel `errTaskMultiParallelNotImplemented` already gone.

### Infra fix (required for the feature)
Concurrent child startup write transactions (workflow sync, run-row inserts) were
upgrade-deadlocking under WAL: two DEFERRED transactions both read then try to
upgrade to write → SQLite returns immediate `SQLITE_BUSY` that busy_timeout cannot
retry. Fix: `_txlock=immediate` in the SQLite DSN (`internal/store/sqlite.go`) so
write transactions `BEGIN IMMEDIATE` and wait on busy_timeout (5s) instead. Driver
`modernc.org/sqlite` v1.49.0 supports `_txlock` (deferred|immediate|exclusive).

### Tests (all `-race` clean)
- Pure: `TestResolveTaskMultiParallelLimit`, `TestAggregateTaskMultiParallelResult`.
- Integration (real git): `...ParallelBoundsConcurrency` (limit 2, 3 children;
  sleep-free — children block in `execute`, a non-blocking channel read proves the
  3rd waits for a slot), `...ParallelFailLate`, `...ParallelCancellation`
  (limit 1: running child canceled + not-started item canceled + repeated cancel),
  `...ParallelEmitsResolvedLimit`.

### Handoff
task_09 renders a real concurrent run (items settle out of order; worktree
metadata on snapshot items; `parallel_limit` only on the `started` event).
task_10 documents the flags, default `2`, fail-late semantics, and worktree-per-child
isolation. See [[task_07]] for the registration/remap layer this builds on.
