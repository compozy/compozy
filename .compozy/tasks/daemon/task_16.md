---
status: completed
title: Daemon Performance Optimizations
type: refactor
complexity: critical
dependencies:
  - task_05
  - task_06
  - task_08
  - task_12
  - task_13
  - task_15
---

# Task 16: Daemon Performance Optimizations

## Overview
Land the P0 and high-value P1 performance fixes surfaced by the cross-cutting analysis under `.compozy/tasks/daemon/perf_analysis/` so that daemon read paths, the event journal, the ACP streaming pipeline, the TUI frame loop, and CLI cold start all meet the budgets captured in each report. Every change must preserve external contracts (event schema, SSE framing, golden CLI output, run.db/global.db row bytes) while eliminating the dominant allocation, syscall, and locking hotspots ahead of formal QA.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and every file under `.compozy/tasks/daemon/perf_analysis/` before touching code
- ACTIVATE `golang-pro` before any Go change, `systematic-debugging` + `no-workarounds` if a fix exposes a root-cause bug, and `testing-anti-patterns` + `cy-final-verify` for the test + closeout steps
- REFERENCE TECHSPEC sections "Daemon runtime", "Transport Contract", "CLI/TUI clients", "Testing Approach", and "Known Risks"; do not duplicate their content here
- BASELINE-BEFORE-CHANGE — capture `go test -bench -benchmem` or `hyperfine` numbers for each target surface before merging its fix and re-run after to prove the win
- PRESERVE ISOMORPHISM — `events.jsonl`, SSE frames, `run.db` / `global.db` row bytes, and `compozy --help|--version|*-help` golden outputs must stay byte-identical unless the fix explicitly rewrites a contract (with TechSpec update)
- ONE LEVER AT A TIME — ship each hotspot as a narrow change so regressions can be bisected; do not bundle storage, streaming, TUI, and CLI wins in a single commit
- NO WORKAROUNDS — if a fix uncovers a real defect (drop amplification, missing sync, leaked goroutine), repair the defect at the root instead of papering over it
- TESTS REQUIRED — every change must add or extend a micro-benchmark or behavior test that encodes the invariant being protected
</critical>

<requirements>
1. MUST resolve the P0 storage findings: push `LIMIT` into `rundb.ListEvents`, drop the redundant in-Go `EventAfterCursor` pass in `RunManager.Events`, batch the N+1 `globalDB.GetWorkflow` inside `RunManager.List`, remove the unused secondary indexes on `rundb.events|job_state|transcript_messages|artifact_sync_log`, and normalize `runs.status` to a canonical lowercase-trimmed form so the `idx_runs_workspace_status` index is actually used.
2. MUST add an LRU-style per-run read-only `rundb.RunDB` cache owned by `RunManager` so `Snapshot`, `Events`, `resolveTerminalState`, and reconcile paths stop opening and closing a fresh handle (plus schema check) per request, with idle TTL + explicit release on `removeActive` and on daemon `Shutdown`.
3. MUST batch the reconcile/purge write paths: wrap purge deletions and `MarkRunCrashed` updates in transactional or `IN (...)` batches, and reuse prepared `*sql.Stmt` objects inside `StoreEventBatch` and the sync reconciler so per-row statement re-parse is eliminated.
4. MUST collapse redundant JSON work on the session-update hot path: marshal the public session-update envelope exactly once and reuse `json.RawMessage` through `runtimeevents.NewRuntimeEvent`, the extension observer dispatch, and the stdout streamer; remove the `reflect.ValueOf` gate in `hasRuntimeEventSubmitter`; and replace `cloneJSONValue` plus the goroutine-per-subscriber fan-out in `extension/dispatcher.go` with a single marshal + bounded worker pool.
5. MUST decouple journal durability from live publish: publish each flushed event to the in-process `Bus` before `file.Sync()` completes (or run fsync on a dedicated ticker), raise `defaultBatchSize` or make it adaptive, and ensure terminal events keep the existing append-before-publish durability guarantee.
6. MUST cut SSE syscalls to one write + one flush per frame by building each SSE frame in a pooled `bytes.Buffer` inside `internal/api/core/sse.go`, wrap stdout encoders in `bufio.Writer` inside `executor/event_stream.go` and `exec/exec.go`, and document the new flush cadence in the transport contract note.
7. MUST eliminate the dominant TUI-frame hotspots: cache per-color `ansiBackgroundSequence` strings and short-circuit `reapplyOwnedBackground` when content is known-owned, skip `transcriptViewport.SetContent` on timeline cache hits, and drive `refreshSidebarContent` from a `sidebarDirty` flag plus a spinner-only update path when nothing else changed.
8. MUST remove the defensive `cloneContentBlocks` deep copy from `transcript.Snapshot`/`buildVisibleEntry` after proving `ContentBlock.Data` is immutable post-construction, promote shared lipgloss styles in `internal/core/run/ui/styles.go` into package-level values, and replace the fresh `techPanelStyle` allocation inside `panelContentWidth`/`sidebarContentWidth` with a constant frame size.
9. MUST trim CLI cold start: defer `startUpdateCheck` past `--help|--version|completion` detection, lazily build the kernel dispatcher via `sync.OnceValue` (and drop the `panic(ValidateDefaultRegistry)` path from the hot entrypoint), and memoize `workspace.Discover` per process invocation.
10. SHOULD land the adjacent P1 wins that fall inside the touched surfaces when they are cheap and in-scope: `runtimeOverrideInput` allocation cleanup, `activeRun.stateMu` → atomics, `Shutdown` drain via `sync.WaitGroup`, `composeSessionPrompt` alloc collapse, and `aggregateUsage` atomics.
11. MUST land every change behind before/after benchmarks or integration assertions that live in-tree (e.g., `internal/daemon/*_bench_test.go`, `internal/store/rundb/*_bench_test.go`, `internal/core/run/journal/*_bench_test.go`, `internal/core/run/ui/*_bench_test.go`, `internal/cli/*_bench_test.go` or `hyperfine` ledger entries under `.codex/ledger/`).
12. MUST keep `make verify` green at the end of the task and MUST NOT disable, skip, or weaken any existing test to achieve the performance numbers.
</requirements>

## Subtasks
- [x] 16.1 Storage layer wins: fix `ListEvents` LIMIT, N+1 workflow join, dead rundb indexes, `runs.status` normalization, prepared-stmt + batch reconcile/purge writes, and writer-pool sizing per `analysis_storage_sqlite.md`.
- [x] 16.2 Daemon runtime wins: introduce the per-run read-handle cache, watcher debounce short-circuit, `activeRun` atomics, and shutdown WaitGroup per `analysis_daemon_runtime.md`.
- [x] 16.3 Streaming pipeline collapse: single-marshal session updates, bufferless observer dispatch → bounded worker pool, `hasRuntimeEventSubmitter` de-reflect, journal publish/fsync decoupling, SSE single-write framing, and bus snapshot de-allocation per `analysis_run_executor.md` + `analysis_event_journal.md`.
- [x] 16.4 TUI frame wins: `sidebarDirty` gating, timeline cache-hit `SetContent` skip, `reapplyOwnedBackground` fast path, `cloneContentBlocks` removal, and lipgloss style memoization per `analysis_tui_rendering.md`.
- [x] 16.5 CLI cold-start wins: defer update-check goroutine, lazy dispatcher + drop-panic, memoized `workspace.Discover`, and golden-output parity checks per `analysis_cli_startup.md`.
- [x] 16.6 Capture before/after evidence (benchmarks + `hyperfine`/pprof ledger entries) and run `make verify` as the final gate.

## Implementation Details
Drive each subtask from the matching report under `.compozy/tasks/daemon/perf_analysis/`. Do not fabricate new architecture — every change is a surgical replacement of the documented hotspot. Cross-cutting hotspots `X1..X5` in `SUMMARY.md` are the canonical priority list; implement them in the "Storage wins → Streaming collapse → TUI frame budget → Cold-start" order called out there so each later measurement sees a clean baseline.

Reference the TechSpec "Daemon runtime", "Transport Contract", "CLI/TUI clients", "Testing Approach", and "Known Risks" sections for the contractual invariants (UDS/HTTP parity, SSE cursor semantics, home-scoped DB ownership, attach-mode defaults) that must remain stable across this performance pass.

### Relevant Files
- `.compozy/tasks/daemon/perf_analysis/SUMMARY.md` — canonical cross-cutting priority list (`X1..X5`) and execution ordering for this task.
- `.compozy/tasks/daemon/perf_analysis/analysis_daemon_runtime.md` — P0/P1 findings for `RunManager` list/events/open-stream, watcher, `activeRun.stateMu`, and `Shutdown`.
- `.compozy/tasks/daemon/perf_analysis/analysis_storage_sqlite.md` — P0/P1 findings for rundb indexes, `ListEvents` LIMIT, N+1 workflow lookup, pool sizing, and reconcile/purge batching.
- `.compozy/tasks/daemon/perf_analysis/analysis_run_executor.md` — P0/P1 findings for session-update marshalling, observer dispatch, `hasRuntimeEventSubmitter` reflection, and stdout streamer buffering.
- `.compozy/tasks/daemon/perf_analysis/analysis_event_journal.md` — P0/P1 findings for journal fsync cadence, replay pagination, SSE frame syscalls, and bus snapshot allocations.
- `.compozy/tasks/daemon/perf_analysis/analysis_tui_rendering.md` — P0/P1 findings for `reapplyOwnedBackground`, timeline cache, sidebar rebuild, `cloneContentBlocks`, and style reuse.
- `.compozy/tasks/daemon/perf_analysis/analysis_cli_startup.md` — P0/P1 findings for update-check goroutine, dispatcher eagerness, workspace discovery, and help-path isomorphism.
- `internal/daemon/run_manager.go` — primary daemon run-lifecycle owner hosting the list/events/snapshot/open-stream hotspots.
- `internal/daemon/watchers.go` — workflow watcher debounce, checksum, and reconcile-state seam flagged by F4.
- `internal/daemon/shutdown.go` — drain loop + purge hotspots (F10, F18) to fix under this task.
- `internal/daemon/reconcile.go` — startup reconcile hotspots (P1-3) that must move to batched updates.
- `internal/store/rundb/run_db.go` — `ListEvents` LIMIT, `StoreEventBatch` prepared statements, projection upserts.
- `internal/store/rundb/migrations.go` — migration that drops the unused secondary indexes on `events`, `job_state`, `transcript_messages`, `artifact_sync_log`.
- `internal/store/globaldb/runs.go` + `internal/store/globaldb/archive.go` + `internal/store/globaldb/registry.go` — `runs.status` normalization and `idx_runs_workflow_id` introduction.
- `internal/store/globaldb/sync.go` — prepared-statement reuse inside sync reconciliation loops.
- `internal/store/sqlite.go` + `internal/store/store.go` — pragma tuning and writer-pool sizing.
- `internal/core/run/journal/journal.go` — fsync/publish decoupling, batch sizing, and encoder reuse.
- `internal/core/run/executor/event_stream.go` + `internal/core/run/exec/exec.go` — stdout encoder buffering and exec stream backpressure.
- `internal/core/run/internal/acpshared/session_handler.go` + `internal/core/run/internal/acpshared/command_io.go` + `internal/core/run/internal/runtimeevents/events.go` — session-update marshal collapse and reflection removal.
- `internal/core/extension/dispatcher.go` — observer dispatch worker pool and single-marshal payload reuse.
- `internal/api/core/sse.go` — single-write SSE framing.
- `pkg/compozy/events/bus.go` — atomic-snapshot fan-out replacing the per-publish slice allocation.
- `internal/core/run/ui/styles.go` + `internal/core/run/ui/timeline.go` + `internal/core/run/ui/sidebar.go` + `internal/core/run/ui/view.go` + `internal/core/run/ui/update.go` — TUI frame-budget hotspots and style reuse.
- `internal/core/run/transcript/model.go` + `internal/core/run/transcript/render.go` — `cloneContentBlocks` removal and allocation-aware truncation.
- `cmd/compozy/main.go` + `internal/cli/root.go` + `internal/cli/commands.go` + `internal/core/kernel/deps.go` + `internal/core/kernel/dispatcher.go` — CLI cold-start deferred update check, lazy dispatcher, and drop-panic path.
- `internal/core/workspace/config.go` — memoized `Discover` result for per-process reuse.

### Dependent Files
- `internal/daemon/run_manager_test.go` + `internal/daemon/service_test.go` + `internal/daemon/reconcile_test.go` + `internal/daemon/watchers_test.go` + `internal/daemon/shutdown_test.go` — must be extended with the new behavior/bench gates.
- `internal/store/rundb/run_db_test.go` + `internal/store/rundb/migrations_test.go` — must cover the LIMIT contract, projection upsert behavior, and the dead-index drop migration.
- `internal/store/globaldb/runs_test.go` + `internal/store/globaldb/archive_test.go` + `internal/store/globaldb/registry_test.go` — must cover the canonical status shape, the new workflow-id index, and batch reconcile semantics.
- `internal/core/run/journal/journal_test.go` — must assert the new fsync cadence and that terminal events retain append-before-publish durability.
- `internal/core/run/executor/event_stream_test.go` + `internal/core/run/internal/acpshared/*_test.go` — must lock the single-marshal session-update contract and the reflection-free submitter gate.
- `internal/core/extension/dispatcher_test.go` — must cover the bounded-worker observer dispatch and the single-marshal payload handoff.
- `internal/api/core/sse_test.go` — must cover the single-write SSE framing contract.
- `pkg/compozy/events/bus_test.go` + `pkg/compozy/events/bus_integration_test.go` — must cover the atomic-snapshot fan-out.
- `internal/core/run/ui/view_test.go` + `internal/core/run/ui/update_test.go` + `internal/core/run/transcript/*_test.go` — must lock the dirty-flag + cache-hit + clone-free invariants.
- `internal/cli/root_command_execution_test.go` + `internal/cli/testdata/*.golden` + `cmd/compozy/main_test.go` (if present) — must prove `--help|--version|completion` output is byte-identical after the deferred update-check.
- `internal/core/workspace/config_test.go` — must assert the memoization behavior and single-invocation guarantee.
- `.compozy/tasks/daemon/_techspec.md` — update only if a fix changes a documented contract (e.g., fsync timing, SSE framing, or help-path side effects).
- `.codex/ledger/` — new ledger entry recording per-hotspot before/after numbers for the benchmarks referenced in subtask 16.6.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — singleton boot + reconcile path is directly impacted by the handle-cache, reconcile-batch, and shutdown fixes.
- [ADR-002: Keep Human Artifacts in the Workspace and Move Operational State to Home-Scoped SQLite](adrs/adr-002.md) — storage wins must keep the human-artifact vs. daemon-state split intact.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — SSE framing, snapshot pagination, and replay cursor semantics must stay contract-identical.
- [ADR-004: Preserve TUI-First UX While Introducing Auto-Start and Explicit Workspace Operations](adrs/adr-004.md) — TUI frame-budget work must protect attach-mode defaults, `tasks run` ergonomics, and keyboard responsiveness.

## Deliverables
- Storage wins: `ListEvents` with SQL `LIMIT`, batched workflow lookup in `RunManager.List`, normalized `runs.status` + canonical predicates, dead rundb indexes dropped via migration, batched purge/reconcile writes.
- Runtime wins: per-run read-handle cache with idle TTL + shutdown-aware release, atomics-based `activeRun` state, watcher debounce short-circuit, drain-aware `Shutdown` via WaitGroup.
- Streaming wins: single-marshal session-update path, bounded-worker observer dispatch, journal publish/fsync decoupling, single-write SSE framing, atomic-snapshot bus fan-out.
- TUI wins: dirty-flag sidebar refresh, timeline cache-hit `SetContent` skip, `reapplyOwnedBackground` fast path, `cloneContentBlocks` removal, promoted package-level lipgloss styles.
- CLI wins: deferred update-check goroutine, lazy dispatcher, drop-panic path, memoized `workspace.Discover`.
- In-tree benchmarks or `hyperfine` ledger entries recording before/after numbers for every landed hotspot.
- Unit tests with 80%+ coverage on every touched file **(REQUIRED)**
- Integration tests covering the journal publish/fsync contract, SSE framing, attach/watch replay, and CLI help isomorphism **(REQUIRED)**
- Green `make verify` run as the final gate **(REQUIRED)**

## Tests
- Unit tests:
  - [x] `rundb.ListEvents` returns at most `limit+1` rows and sets `HasMore` correctly at the boundary.
  - [x] `RunManager.List` issues exactly one `globalDB` lookup regardless of active-run count (counter instrumentation or fake store).
  - [x] `RunManager` per-run RunDB cache serves concurrent `Snapshot`/`Events` calls from a single handle and closes idle handles after the configured TTL.
  - [x] `globaldb` `runs.status` predicates use equality/`IN` against canonical values and the `idx_runs_workspace_status` index (assert via `EXPLAIN QUERY PLAN` helper).
  - [x] `rundb` migration drops only the flagged dead indexes and leaves every still-used index in place.
  - [x] `journal.Submit` → terminal event path still publishes after a successful `file.Sync()` while non-terminal events publish after `writer.Flush()` without waiting on fsync.
  - [x] `session.update` payload is marshalled exactly once per event across hook dispatch, runtime-event, and stdout streamer (observed via an instrumented `json.Marshal` hook).
  - [x] `extension.dispatcher` bounded worker pool never exceeds `runtime.GOMAXPROCS` concurrent observer invocations and shares one `json.RawMessage` across subscribers.
  - [x] `api/core.WriteSSE` performs a single `Write` + `Flush` per frame for a representative payload.
  - [x] `events.Bus.Publish` path does not allocate a fresh subscriber slice when topology is unchanged.
  - [x] `ui` `refreshSidebarContent` is skipped on tick when the `sidebarDirty` flag is clear and no job is running.
  - [x] `ui.renderTimelinePanel` does not call `SetContent` on a cache hit (confirmed via an instrumented viewport double).
  - [x] `transcript.Snapshot` no longer deep-copies `ContentBlock.Data` and the resulting snapshot is still safe for read-only consumers.
  - [x] `startUpdateCheck` is not invoked when the detected command is `help`, `version`, or a completion flow.
  - [x] `workspace.Discover` returns the memoized result on repeated calls within the same process invocation.
- Integration tests:
  - [x] `compozy runs list` with 100 seeded runs completes in a single `globalDB` round-trip and returns the same run set as before the optimization.
  - [x] Attach/replay of a 10k-event run streams every event in order with the new SQL `LIMIT` pagination and no duplicated or lost events.
  - [x] Daemon shutdown drains N active runs in parallel and reports per-run drain status even when `waitCtx` fires mid-shutdown.
  - [x] Observer-subscribing extension receives every published payload exactly once under a 2k-event burst, without goroutine-count blow-up.
  - [x] SSE golden dump for a canned run matches the pre-optimization byte stream (IDs, event names, data payloads) after the single-write framing change.
  - [x] TUI golden render at fixed terminal sizes stays byte-identical across sidebar, timeline, and root wrappers after the frame-budget fixes.
  - [x] `compozy --help`, `compozy --version`, and `compozy completion bash` golden outputs are byte-identical before and after the CLI cold-start changes.
  - [x] `make verify` passes cleanly after the full performance pass.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Every P0 finding in the perf_analysis reports has a landed fix with measured before/after evidence
- Storage, streaming, TUI, and CLI hotspots documented in `SUMMARY.md` are demonstrably improved by the targets listed in their respective analyses (no regressions on untouched benchmarks)
- `events.jsonl`, SSE frame bytes, `run.db` / `global.db` row bytes, and CLI help goldens remain byte-identical (or the TechSpec is updated to document an intentional change)
- `make verify` passes cleanly on the final commit of the task
