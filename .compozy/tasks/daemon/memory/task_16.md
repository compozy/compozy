# Task Memory: task_16.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Land the P0 and high-value P1 performance fixes documented under `.compozy/tasks/daemon/perf_analysis/` without changing external contracts (event schema, SSE framing, golden CLI output, `run.db`/`global.db` row bytes).
- Success required one surgical change per hotspot with before/after benchmark or `hyperfine` evidence and a clean `make verify` at the end.

## Important Decisions
- Keep the implementation order aligned with `perf_analysis/SUMMARY.md`: storage/runtime first, then streaming, then TUI, then CLI cold-start, so later measurements are not masked by earlier hotspots.
- Treat the daemon-backed exec completion race as a root-cause contract bug: the CLI must wait for the durable terminal snapshot after the streamed terminal event because `scope.Close()` still owns extension shutdown/audit flushes.
- Preserve the public extension hook schema while swapping the internal `agent.on_session_update` payload carrier to `json.RawMessage` so the public session update is marshalled once and reused across observer dispatch plus runtime-event persistence.

## Learnings
- Storage/runtime wins landed: `rundb.ListEvents` now pushes SQL `LIMIT`, `RunManager.List` batches workflow slug lookup, `RunManager.Events` replays paginated SQL pages without an extra `EventAfterCursor` pass, `RunManager` caches read-only `RunDB` handles with idle eviction, and purge/reconcile paths batch writes through `MarkRunsCrashed` / `DeleteRuns`.
- Status normalization landed in `globaldb`: writes canonicalize run status, queries now use equality/`IN`, and the migration rebuilds `idx_runs_workspace_status` plus adds `idx_runs_workflow_id`.
- Streaming/journal fixes landed: session updates marshal the public update once and reuse raw JSON through hooks plus runtime events, observer dispatch uses a bounded worker pool with shared `json.RawMessage`, stdout streamers use buffered writers, SSE frames write once per flush, and the journal publishes non-terminal batches before fsync while preserving terminal append-before-publish durability.
- TUI/CLI fixes landed: ANSI background sequences and panel frame sizes are cached, sidebar/timeline refreshes honor dirty/cache-hit fast paths, transcript snapshots stop deep-copying immutable block bytes, update checks are skipped for help/version/completion, dispatcher construction is lazy, and `workspace.Discover` is memoized per process.
- Verified before/after evidence:
  - `BenchmarkRunDBListEventsFromCursor`: `3,073,760 ns/op`, `4,883,753 B/op`, `80,048 allocs/op` → `162,678 ns/op`, `241,640 B/op`, `4,134 allocs/op`
  - `BenchmarkRunManagerListWorkspaceRuns`: `893,060 ns/op`, `274,764 B/op`, `7,701 allocs/op` → `349,162 ns/op`, `179,525 B/op`, `4,492 allocs/op`
  - `hyperfine` means: `--help` `10.0 ms` → `7.8 ms`; `--version` `9.9 ms` → `6.4 ms`; `completion bash` `9.2 ms` → `6.7 ms`
- Final verification passed with `make verify`.

## Files / Surfaces
- `internal/daemon/{run_manager.go,reconcile.go,shutdown.go}`
- `internal/store/{sqlite.go,store.go}`
- `internal/store/globaldb/{runs.go,archive.go,registry.go,sync.go,migrations.go}`
- `internal/store/rundb/{run_db.go,migrations.go}`
- `internal/core/run/journal/journal.go`
- `internal/core/run/internal/acpshared/{session_handler.go,command_io.go}`
- `internal/core/extension/dispatcher.go`
- `internal/api/core/sse.go`
- `pkg/compozy/events/bus.go`
- `internal/core/run/ui/{styles.go,timeline.go,update.go,view.go,sidebar.go}`
- `internal/core/run/transcript/{model.go,render.go}`
- `internal/cli/root.go`
- `cmd/compozy/main.go`
- `internal/core/workspace/config.go`

## Errors / Corrections
- The daemon-backed exec CLI originally returned after the streamed terminal event, which raced extension shutdown and audit durability. `waitForDaemonRunTerminal` now polls the daemon snapshot until the run row is terminal, and a focused regression test covers the late-terminal-snapshot sequence.
- The initial session-update optimization still re-encoded the observer-hook update payload. The fix widened the internal hook carrier to `json.RawMessage` and added a marshal-count test so the public update is encoded once.

## Ready for Next Run
- Task 16 is complete, verified, and committed locally as `cc31d95` (`perf: optimize daemon performance hot paths`). The remaining unstaged changes are workflow-memory / task-tracking artifacts only.
