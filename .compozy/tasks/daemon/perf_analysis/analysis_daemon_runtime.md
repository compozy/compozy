# Daemon Runtime — Performance Analysis

Scope: `/Users/pedronauck/Dev/compozy/looper/internal/daemon/` (run_manager.go, watchers.go, service.go, boot.go, reconcile.go, transport_mappers.go, `*_transport_service.go`, extension_bridge.go, host.go, shutdown.go, lock.go, run_snapshot.go).

Methodology note: no profiler was run. Findings below are static/architectural hotspots backed by line-level reads. Each item names concrete bench targets in the Verification Plan so p50/p95/throughput claims can be confirmed (or rejected) before any fix is merged. No code changes were made.

## Summary

- `RunManager.List` performs one `GlobalDB.GetWorkflow` per row via `toCoreRun` — a textbook N+1 that scales list latency with active-run count and thrashes SQLite under a read-heavy Status/List pattern.
- `RunManager.Events` loads the full post-cursor event tail from SQLite, then re-filters and allocates a second slice in Go. For long-lived runs with many replays (attach/watch) this is the dominant allocation/latency hot path and it cannot be bounded by `limit` until after the full DB row set is materialized.
- `OpenStream` opens a fresh `rundb.RunDB` handle for every subscriber (replay runs through `Events` → `openRunDB` twice), and every synchronous service call (`Snapshot`, `Events`, terminal-state resolution) opens + closes its own SQLite connection. Open/close cycles dominate attach latency.
- Workflow watcher path does a full `filepath.WalkDir` + sha256 of every changed file inline in the debounce flush goroutine, blocking the fsnotify loop and ballooning CPU when a single sync emits many `write` events on large artifacts.
- `activeRun.stateMu` uses an RWMutex, but every taken lock in the hot path is a `Lock()` write (cancel/setWatcher/setCloseTimeout) or a `RLock()` of a one-int/one-bool field. The RWMutex overhead (two atomic ops + bookkeeping) is net-negative vs a plain `sync.Mutex` for fields this small.

---

## P0 Findings

### F1. N+1 workflow lookup inside `RunManager.List`
- File: `internal/daemon/run_manager.go:382-391`, `run_manager.go:1279-1289`
- Pattern: `List` iterates the `rows` slice and calls `m.toCoreRun(...)` on each one. `toCoreRun` unconditionally dereferences `row.WorkflowID` and issues one `m.globalDB.GetWorkflow(ctx, workflowID)` per row (a full `SELECT ... FROM workflows WHERE id = ?` round-trip against SQLite).
- Impact: For the default `defaultRunListLimit = 100` a single list call issues up to 101 SQLite queries (1 list + 100 workflow lookups). Status dashboards, TUI refresh loops, and the remote attach flow all call `List`. With 100 active runs and workflow lookup ~50µs each, that is ~5ms of synchronous DB time per call against a single-writer SQLite DB — and it contends with the single DB pool that also serves `PutRun`/`UpdateRun` on the hot write path. Every run insert/update therefore sees extra queue pressure during list refresh.
- Fix: Batch lookups. Collect the unique `workflow_id` set after reading `rows`, issue one `SELECT id, slug FROM workflows WHERE id IN (?,?,...)` via a helper `ListWorkflowsByIDs`, cache the `map[id]slug` once, then map rows to `apicore.Run` without re-querying:
  ```go
  ids := collectWorkflowIDs(rows) // dedup
  slugs, _ := m.globalDB.WorkflowSlugsByIDs(ctx, ids)
  for i := range rows {
      run := runFromRow(rows[i])
      if rows[i].WorkflowID != nil {
          run.WorkflowSlug = slugs[*rows[i].WorkflowID]
      }
      result = append(result, run)
  }
  ```
  Long-term: denormalize `workflow_slug` onto `runs` and avoid the join entirely — slug is immutable per workflow ID.
- Expected: `List` p95 drops from O(N) DB calls to O(1). For 100 active runs with an ~50µs workflow lookup, expect ~5ms → ~100µs (50× speedup on DB calls) and a measurable reduction in SQLite write-queue stalls under mixed read/write load.

### F2. `RunManager.Events` loads unbounded event tail, filters in Go, then double-allocates slice
- File: `internal/daemon/run_manager.go:471-527`, backed by `internal/store/rundb/run_db.go:266-315`
- Pattern: `Events` calls `runDB.ListEvents(ctx, query.After.Sequence)` which returns **every** event with `sequence >= fromSeq` (no LIMIT). It then allocates `filtered := make([]Event, 0, len(events))` and walks the whole list calling `apicore.EventAfterCursor` before slicing to `limit`. When the caller asks for 256 events but 10 000 exist past the cursor, all 10 000 rows are pulled into memory, decoded, re-filtered, and copied.
- Impact: Every attach reconnect replays via this path. Each Event has a `json.RawMessage` payload — a 1 KB average payload × 10 000 events = 10 MB of per-request heap churn. The `filtered[:limit]` copy on the hot path (`append(page.Events, filtered[:limit]...)`) allocates a second backing array. For long-running workflows (hours), an attach hits tens of thousands of events on first connect. This is the #1 attach-latency contributor.
- Fix:
  1. Push `LIMIT` into SQL. Add `ListEventsRange(ctx, fromSeq, limit+1)` to `rundb` that emits `ORDER BY sequence ASC LIMIT ?`. Use `limit+1` to set `HasMore` without a second query.
  2. Skip the in-Go `EventAfterCursor` re-filter: `sequence >= fromSeq` already upper-bounds the set; anything still failing `EventAfterCursor` is a bug in the cursor encoding, not a filter to run at read time.
  3. Avoid the second slice. Reassign `page.Events = filtered[:limit]` in place; no copy.
  ```go
  rows, err := runDB.ListEventsRange(ctx, query.After.Sequence, limit+1)
  page := apicore.RunEventPage{Events: rows}
  if len(rows) > limit {
      page.HasMore = true
      page.Events = rows[:limit]
  }
  if n := len(page.Events); n > 0 {
      c := apicore.CursorFromEvent(page.Events[n-1])
      page.NextCursor = &c
  }
  ```
- Expected: Attach replay allocs drop from O(events-past-cursor) to O(limit). For a 10 000-event run with `limit=256`, a ~40× reduction in DB rows scanned and a ~40× reduction in heap churn per page. Attach p95 falls from "tens of ms" to roughly constant per page regardless of run age.

### F3. SQLite handle churn: open+close per request on every read surface
- File: `internal/daemon/run_manager.go:414-420`, `482-488`, `1549-1555`, `1563-1569`; `reconcile.go:182-188`
- Pattern: `Snapshot`, `Events`, `resolveTerminalState`, and `appendSyntheticCrashEvent` each call `openRunDB(ctx, runID)` then `defer runDB.Close()`. There is no pool or cache. `rundb.Open` constructs a `*sql.DB`, runs schema migrations / PRAGMAs, and allocates new prepared statement caches per open.
- Impact: For every active run there is one live bus subscription path plus N stateless queries per attach + watch + status tick. A naive TUI that polls `Events` every 250 ms for 10 active runs = 40 Open/Close pairs/sec. Each pair is an fsync-heavy operation (SQLite journal, WAL header validation) — easily 1–5 ms on consumer SSDs, so the daemon burns measurable CPU on handle bring-up/tear-down that serves zero user value. It also defeats SQLite WAL's own page cache — the OS page cache stays hot, but prepared statement caches and per-connection memory reset on every Open.
- Fix: Introduce a per-run `*sql.DB` cache keyed on `runID`, owned by `RunManager`, with:
  - Bounded LRU (e.g. `max=64`), evict closes the handle.
  - Idle expiry (e.g. 30 s) via a single background janitor goroutine that is canceled on `Shutdown`.
  - Reference-counted borrow so concurrent `Events` + `Snapshot` share one handle.
  ```go
  db, release, err := m.runDBs.Acquire(ctx, runID)
  defer release()
  ```
  For the terminal-resolution path (`finishRun`), keep the handle already held by the active run's scope for the lifetime of the run and close on `removeActive`.
- Expected: 40 Open/Close pairs/sec → 0 for warm runs. Per-call latency drops by the 1–5 ms SQLite-open cost. Reduces syscall rate (stat, open, close, fstat, ioctl for WAL) visible in `strace -c`.

---

## P1 Findings

### F4. Workflow watcher does blocking WalkDir + sha256 inline on the debounce goroutine
- File: `internal/daemon/watchers.go:118-158`, `239-304`, `455-527`
- Pattern: On every debounce fire, `flushPendingChanges` calls `syncFn` (full sync) and then `reconcileWatchState` which calls `discoverWorkflowWatchDirs` → `filepath.WalkDir(root, ...)` every flush regardless of whether directories actually changed. Then `emitPendingChanges` iterates `changes` and calls `artifactChecksum(root, relPath)` for each, which does a blocking `os.ReadFile` + `sha256.Sum256`. All of this runs on the same goroutine as the fsnotify event loop select (via `<-debounce.channel()` path at line 153).
- Impact: For a workflow root with 200 files, the watcher loop is pinned in WalkDir for tens of ms per flush. A single `git pull` that writes 50 files causes one `syncFn` call (slow), one WalkDir (slow), then 50 sha256 reads serially. During that window the watcher cannot consume new `watcher.Events` — fsnotify's kernel-side queue is bounded and events can be dropped silently when the consumer stalls.
- Fix:
  1. Short-circuit `reconcileWatchState` when `refreshWatches == false`. Today the code at line 261 always calls it; gate it on the flag already tracked in `state.refreshWatches` (the current stopAndFlush path sets it, but the happy-path flush runs it unconditionally).
  2. Do checksum computation in a small worker pool with a bounded work queue, OR compute lazily on the consumer side. The daemon's only use of `Checksum` is `ArtifactUpdatedPayload.Checksum` — emitting with an empty checksum and letting the consumer diff-by-sequence is viable for most TUI uses.
  3. Replace `sha256` with a cheaper hash for this use case (`xxhash` / `fnv64`) — we only need change detection, not crypto.
- Expected: Watcher flush drops from O(total files) to O(changed files). For 50-file writes, sha256 → xxhash is ~8× faster per byte and the serial chain can be parallelized. The fsnotify-event-drop window shrinks from tens of ms to ~sub-ms for typical changes.

### F5. Unbounded growth path: `filtered := make([]Event, 0, len(events))` in `Events`
- File: `internal/daemon/run_manager.go:503-508`
- Pattern: Uses `len(events)` as the capacity hint for the filtered slice even though the final slice is clamped to `limit` (≤1000). For large event tails the allocator reserves capacity for the entire tail.
- Impact: 10k-event tail → 10k Event capacity × struct size (≈56 B plus payload pointer) = ~600 KB of unused capacity per request, GC'd after response write.
- Fix: Cap the hint at `limit+1`. This composes cleanly with F2's SQL-side LIMIT (after F2 is applied, this problem is gone).
- Expected: ~40× smaller allocation for typical replay page; measurable in `-memprofile` allocation totals of the `Events` endpoint.

### F6. `runtimeOverrideInput` uses pervasive pointer-to-value for optional JSON fields
- File: `internal/daemon/run_manager.go:102-131`
- Pattern: Every optional override is `*bool`, `*int`, `*string`, `*float64`, `*[]string`. Decoding this with `encoding/json` forces heap allocation for each present field. The struct has ~20 pointer fields; each `Start*Run` call decodes one.
- Impact: Not the tallest pole, but under burst start (e.g., `compozy reviews start` with batched runs), every start allocates dozens of tiny `*bool`/`*int` values on the heap. This is pure JSON-shape overhead.
- Fix: Replace pointer-optional with the "zero-vs-absent" pattern via `json.RawMessage` per field, or use a helper struct with `bool` + `has_*` flags, or adopt `encoding/json/v2`'s `OmitEmpty`/`omitzero` when available. The cleanest path is to keep the wire format but decode into a stack-based value type using a custom `UnmarshalJSON` that tracks "present" via a bitmask.
- Expected: ~20 heap allocs/start removed. Multiply by run start rate to size the win — typically seen in allocation flame graphs, rarely in latency.

### F7. `activeRun.stateMu` uses `sync.RWMutex` for single-field guards
- File: `internal/daemon/run_manager.go:97-100`, and callers at `run_manager.go:1342-1410`
- Pattern: `stateMu sync.RWMutex` protects `cancelRequested bool`, `closeTimeout time.Duration`, and `watcher *workflowWatcher`. Every access takes the lock, but the reads (`cancelWasRequested`, `currentCloseTimeout`) dereference a single field.
- Impact: `sync.RWMutex` costs roughly 2× a `sync.Mutex` for read-locked, uncontended paths because of its additional atomic writer-counter accounting. Single-field reads are better served by `atomic.Bool`/`atomic.Int64`.
- Fix:
  - `cancelRequested` → `atomic.Bool` (the `markCancelRequested` CAS becomes `CompareAndSwap(false, true)`).
  - `closeTimeout` → `atomic.Int64` storing nanoseconds; `setCloseTimeout` uses a CAS loop to enforce the "only grow" invariant.
  - `watcher` → protect with a small `sync.Mutex` dedicated to that field OR use `atomic.Pointer[workflowWatcher]`.
  - Drop `stateMu` entirely.
- Expected: Removes one RWMutex per active run; turns 6 hot-path lock ops into 6 atomic loads/CAS. For cancel-heavy workloads (shutdown, watcher stop) this shows up in mutex contention profiles. Typical expected win is small absolute, but a clean correctness + perf win together.

### F8. `RuntimeConfig.Clone` copies slices twice per start, plus `validateDaemonRuntimeConfig` clones again
- File: `internal/daemon/run_manager.go:859-861` and `2026-2044`; `internal/core/model/task_runtime.go:89-97`
- Pattern: `startRun` does `runtimeCfg := spec.runtimeCfg.Clone()`, then `validateDaemonRuntimeConfig` does another `check := cfg.Clone()` before validating. Every clone copies `AddDirs` via `append([]string(nil), ...)` and `TaskRuntimeRules` via `CloneTaskRuntimeRules`.
- Impact: Two full deep copies per start on the `TaskRuntimeRules` slice, which for workspaces with many rules is the dominant start-time alloc. `validateDaemonRuntimeConfig` only needs a shallow "RunID blanked" view, not a deep copy.
- Fix: In `validateDaemonRuntimeConfig`, swap `check := cfg.Clone()` for a stack-level shallow copy: `check := *cfg; check.RunID = ""` (slices are shared safely because we only read). Only clone if validation could mutate — it doesn't.
- Expected: Halves the clone cost per start. Visible on `compozy tasks start` micro-benchmark.

### F9. `streamLiveRunEvents` polls `bus.DroppedFor` on every select iteration
- File: `internal/daemon/run_manager.go:1503-1537`
- Pattern: The for-select loop calls `subscription.bus.DroppedFor(subscription.subID)` at the top of every iteration. `DroppedFor` takes `Bus.mu.RLock()` (see `pkg/compozy/events/bus.go:128`) and does a map lookup.
- Impact: For a fast-streaming run (many events per second), every event triggers an extra `RLock + map[SubID]lookup` plus an atomic load on the dropped counter. The dropped counter should only be checked once the publisher has actually reported a drop — but here it runs unconditionally.
- Fix: Move the drop check to the drop-event path: the `subscription` exposes `dropped atomic.Uint64`. Cache a local `lastDropped uint64` and compare `sub.dropped.Load() != lastDropped` outside the bus mutex. Even better, have the bus push a sentinel/overflow event through the channel when it drops, so consumers never poll. Bus API today already tracks drops per-sub — promote drop signaling to the channel rather than a side poll.
- Expected: Removes one `Bus.mu.RLock` per streamed event. For 1k events/sec that is 1000 fewer RLocks per second per subscriber. Reduces bus-mutex contention as subscribers scale.

### F10. `Shutdown` drain loop fans out cancels and then sequentially waits
- File: `internal/daemon/shutdown.go:71-87`
- Pattern: After canceling all active runs, the shutdown loops over `activeRuns` and does `select { case <-run.done; case <-waitCtx.Done() }` sequentially. If run[0] takes 29s to drain (drain timeout = 30s), runs[1..N] are not watched until then; the first `waitCtx.Done()` short-circuits the rest entirely.
- Impact: With N slow-draining runs, the `waitCtx` budget is consumed by whichever run happens to be at index 0 in the map-iteration order. In practice all runs drain in parallel (their goroutines are already running), so the wall-clock is bounded by `max(drain_time)`. But: if `waitCtx.Done()` fires while watching run[0], we return `nil` without confirming runs[1..N] finished — we never observe their `done` channel, never record whether they drained cleanly, and there is no log trail.
- Fix: Use a `sync.WaitGroup` driven by per-run watcher goroutines:
  ```go
  var wg sync.WaitGroup
  for _, run := range activeRuns {
      wg.Add(1)
      go func(r *activeRun) { defer wg.Done(); select { case <-r.done: case <-waitCtx.Done(): } }(run)
  }
  waitCh := make(chan struct{})
  go func() { wg.Wait(); close(waitCh) }()
  select { case <-waitCh: case <-waitCtx.Done(): }
  ```
  Correctness win (every run observed) + a tiny perf win (no serialization of already-parallel work).
- Expected: Same throughput, but correct observability on partial drain; unblocks future work where Shutdown must report which runs timed out.

---

## P2 Findings / Cleanup

### F11. `List` `toCoreRun` trims and re-allocates the workflow ID string
- File: `internal/daemon/run_manager.go:1279-1289`
- Pattern: `workflowID := strings.TrimSpace(*row.WorkflowID)` is done per row even though the globaldb-produced row should already be trimmed. Same pattern at `run_manager.go:395-399`.
- Impact: Small — one allocation per row only if the input has trailing whitespace (the function actually returns the original string when already trimmed; the hit is only a bounds check per row). Still a cleanup.
- Fix: Trust the row shape from `globaldb` (enforce trimming at insert, drop the trims on read). Or precompute `row.WorkflowID` as a non-pointer trimmed string at the store boundary.

### F12. `transportSyncResult` always allocates zero-length slices
- File: `internal/daemon/transport_mappers.go:34-60`
- Pattern: `out.SyncedPaths = append([]string(nil), result.SyncedPaths...)` and `out.Warnings = append([]string(nil), ...)` always allocate a new backing array even for nil/empty source slices.
- Impact: Two extra allocations per sync response in the common "nothing changed" case.
- Fix:
  ```go
  if len(result.SyncedPaths) > 0 { out.SyncedPaths = append([]string(nil), result.SyncedPaths...) }
  if len(result.Warnings) > 0 { out.Warnings = append([]string(nil), result.Warnings...) }
  ```

### F13. `cloneStringMap` in review transport runs per overlay entry; zero-value path already returns nil
- File: `internal/daemon/review_exec_transport_service.go:325-334`
- Pattern: Called once per discovered review provider in `buildWorkspaceReviewRegistry` (line 204). Fine today, but for workspaces with many providers this is repeat map allocs on a cold path.
- Impact: Negligible unless provider counts explode.
- Fix: Keep as-is; note for a future audit. Alternatively share the map since it's treated read-only downstream.

### F14. `Metrics` allocates a fresh `fmt.Sprintf` format on every scrape
- File: `internal/daemon/service.go:114-134`
- Pattern: `fmt.Sprintf(...)` builds the Prometheus text body on every scrape, with 4 `%d` substitutions and a multi-line format string. Allocates a new byte slice each call.
- Impact: Prometheus scrapes at 5-15 s intervals — allocation overhead is trivial. But the code style hides that this is a tiny hot path for an external-facing endpoint.
- Fix: Use `strings.Builder` pre-sized, or `fmt.Fprintf` against a pooled `bytes.Buffer`. Optional.

### F15. `looksLikeWorkflowDir` uses `filepath.Glob` on every sync without caching
- File: `internal/daemon/sync_transport_service.go:98-111`
- Pattern: Every inbound `Sync` call with a path does `filepath.Glob(filepath.Join(path, "task_*.md"))` which scans the directory fresh.
- Impact: One stat + readdir per sync call. Acceptable for the frequency; not a hot path.
- Fix: None immediately; document as a known cost.

### F16. `reserveRunDirectory` uses `os.Mkdir` without pre-checking, then translates `os.ErrExist`
- File: `internal/daemon/run_manager.go:1423-1438`
- Pattern: Cheap. Correct. Flagged only because SQLite `PutRun` below it can also fail-and-race with the directory; the current code already compensates with `cleanupRunDirectory`.
- Impact: Correct.
- Fix: None.

### F17. Lock file write-sync fsync per PID update
- File: `internal/daemon/lock.go:167-185`
- Pattern: `writeLockPID` opens, writes, syncs. Called twice per daemon lifecycle (acquire + release). Info write (`info.go:97-117`) does fsync + directory fsync on every port update and state change.
- Impact: `Info` is rewritten on `SetHTTPPort`, `MarkReady`, and `Close`. That's a handful of fsyncs per daemon boot — fine. Flagged because `SetHTTPPort` happens synchronously during startup and is on the critical path.
- Fix: None immediately.

### F18. `Purge` serial `os.RemoveAll` per candidate
- File: `internal/daemon/shutdown.go:109-127`
- Pattern: Iterates purge candidates and does `os.RemoveAll(runArtifacts.RunDir)` + `DeleteRun` sequentially. Each RemoveAll walks the run's filesystem tree.
- Impact: Purge is periodic and not latency-critical, but with `KeepMax=200` and lots of history, a purge sweep can take seconds and hold an implicit SQLite write lock per delete.
- Fix: Parallelize `RemoveAll` with a bounded worker pool (e.g. `semaphore(8)`), then issue one batched `DELETE FROM runs WHERE run_id IN (...)` at the end. Or make `DeleteRun` async with compensation on failure. Not critical until purge becomes a visible stall.

### F19. `ensureSQLiteDatabaseFile` opens & reads header per reconcile candidate
- File: `internal/daemon/reconcile.go:219-239`
- Pattern: On startup reconcile, for every interrupted run, `ensureSQLiteDatabaseFile` opens the run DB file, reads 16 bytes, and closes it. Then `openRunDB` opens it again.
- Impact: Reconcile runs at most once per boot and over a bounded count. Double-open is one extra stat+open per interrupted run — negligible.
- Fix: None (fold header check into Open if ever hot).

### F20. `activeSnapshot` allocates a new slice under RLock
- File: `internal/daemon/shutdown.go:130-139`, `run_manager.go:1296-1300`
- Pattern: Fine and idiomatic.
- Impact: None meaningful.
- Fix: None.

---

## Verification plan

1. **Baseline (before any change)**
   - Bench `List`: start 100 runs against an in-memory `globaldb`, measure `b.N` `List` calls. Expected to show flat-per-run DB query count.
     ```
     go test -bench=BenchmarkRunManagerList -benchmem ./internal/daemon/
     ```
     Capture `allocs/op` and `ns/op`. For SQLite round-trip visibility, set `PRAGMA journal_mode=WAL` and record with `-cpuprofile`.
   - Bench `Events`: seed a run with 10k events, call `Events(limit=256)` with varying `After.Sequence` values. Measure allocations and wall time.
   - Bench `OpenStream` attach: simulate attach to an active run with 5k existing events, measure time to first streamed event and to finish replay.

2. **Profile**
   - CPU: `go test -bench=BenchmarkRunManagerStream -cpuprofile=cpu.out ./internal/daemon/ && go tool pprof -http=:0 cpu.out` — expect `Bus.DroppedFor` / `globaldb.GetWorkflow` / `rundb.ListEvents` on top.
   - Allocs: `-memprofile=mem.out -benchmem`; look for `runtimeOverrideInput` field allocs, `filtered := make(..., len(events))`, `RuntimeConfig.Clone`.
   - Goroutines / contention: `go test -race -blockprofile=block.out` while running a simulated shutdown with 50 active runs; inspect `activeRun.stateMu` contention. Expected to flag RWMutex vs Mutex (F7).

3. **After each fix, one lever at a time**
   - Re-run the same benches; require ≥2× improvement on the target metric before merging. Capture `benchstat` diff between baseline and fix.
   - Golden outputs:
     - `RunManager.List` against a fixture with 20 workspace/workflow/run triples → deterministic JSON.
     - `RunManager.Events` for a run seeded with 500 events, 3 cursor positions → deterministic JSON.
     - `Snapshot` for a run with jobs + transcript + usage rows.
     Compare `sha256sum` of pretty-printed JSON before and after each fix; any change to output means isomorphism is broken and the fix must be revised.

4. **Integration measurements**
   - Start a real daemon, drive `compozy runs list` in a loop (100 iterations) and measure wall time via `hyperfine --warmup 3 --runs 10 'compozy runs list'`. Repeat after F1+F3 are merged.
   - Run `compozy runs attach <id>` against a 10k-event run, measure time-to-first-event with a small test client. Repeat after F2+F3.
   - Chaos: trigger `SIGTERM` against a daemon with 20 active runs and verify `Shutdown` drain telemetry is complete (F10).

5. **Regression gate**
   - Each fix gets one commit with `benchstat` output and `sha256sum -c golden_checksums.txt` both in the commit body.
   - `make verify` must pass at every step (project gate).
