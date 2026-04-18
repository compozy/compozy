Goal (incl. success criteria):

- Complete daemon task `16` (`Daemon Performance Optimizations`) end to end.
- Success means: land the required storage/runtime/streaming/TUI/CLI performance fixes called out in `.compozy/tasks/daemon/perf_analysis/`, preserve documented transport/storage/golden-output contracts, capture before/after evidence, pass required tests, and finish with a clean `make verify`.

Constraints/Assumptions:

- Must follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/daemon/{task_16.md,_techspec.md,_tasks.md}`, ADRs `001`-`004`, and the required workflow memory files.
- Required skills loaded for this run: `cy-workflow-memory`, `cy-execute-task`, `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, and `cy-final-verify`.
- The worktree is already dirty in task-tracking and ledger files unrelated to this implementation; do not revert or overwrite unrelated changes.
- Need benchmark/timing evidence before and after each landed hotspot area; `make verify` is the final gate before any completion or commit.

Key decisions:

- Keep the implementation order aligned with `perf_analysis/SUMMARY.md`: storage/runtime first, then streaming, then TUI, then CLI cold-start, so later measurements are not masked by earlier hotspots.
- Treat the daemon-backed exec completion race as a root-cause synchronization bug: the CLI must wait for the durable terminal snapshot after the streamed terminal event because `scope.Close()` still owns extension shutdown and audit durability.
- Preserve the public extension hook schema while switching the internal session-update hook carrier to `json.RawMessage` so the public update is marshalled once and reused through hooks plus runtime events.

State:

- Completed and locally committed as `cc31d95` (`perf: optimize daemon performance hot paths`).

Done:

- Read workspace instructions, required skill files, workflow memory, task specification, task list, techspec sections, and ADRs `001`-`004`.
- Scanned related daemon ledgers for cross-agent context.
- Confirmed the current worktree has unrelated tracking-file edits that must be preserved.
- Landed the storage/runtime fixes:
  - `rundb.ListEvents` now applies SQL `LIMIT`/`limit+1` pagination semantics and `StoreEventBatch` reuses prepared statements.
  - `globaldb` now canonicalizes run status on write, queries use equality/`IN`, and migrations rebuild `idx_runs_workspace_status`, add `idx_runs_workflow_id`, and drop the dead `rundb` secondary indexes.
  - `RunManager.List` batches workflow slug lookup, `RunManager` caches read-only `RunDB` handles with idle eviction, `Events` replays paginated SQL pages directly, reconcile uses batched `MarkRunsCrashed`, and purge uses batched `DeleteRuns`.
- Landed the streaming/journal fixes:
  - public session updates marshal once and reuse `json.RawMessage` through the observer hook payload and runtime-event envelope;
  - `hasRuntimeEventSubmitter` is reflection-free;
  - extension observer dispatch uses a bounded worker pool with shared raw payload;
  - journal non-terminal batches publish before fsync while terminal events keep append-before-publish durability;
  - SSE frames write/flush once; exec/executor stdout streams use `bufio.Writer`;
  - `events.Bus.Publish` uses an atomic steady snapshot without per-publish subscriber-slice copies.
- Landed the TUI/CLI fixes:
  - cached ANSI background sequences and shared panel frame sizes;
  - sidebar dirty-flag refresh and timeline cache-hit `SetContent` skip;
  - transcript snapshots stop deep-copying immutable block bytes;
  - CLI help/version/completion skip update-check startup;
  - dispatcher construction is lazy and no longer panics on the hot path;
  - `workspace.Discover` is memoized per process.
- Added/updated focused regression coverage for the perf invariants, including:
  - run-db SQL limit + `HasMore`;
  - single workflow slug lookup;
  - run-db cache reuse/eviction;
  - terminal snapshot wait after streamed terminal event;
  - single-marshalled session-update raw reuse;
  - bounded observer workers + shared payload;
  - single-write SSE framing;
  - steady-topology bus publish allocations;
  - sidebar/timeline/transcript fast paths;
  - help/version/completion update-check skip;
  - workspace discovery memoization.
- Captured before/after evidence:
  - `BenchmarkRunDBListEventsFromCursor`: `3,073,760 ns/op`, `4,883,753 B/op`, `80,048 allocs/op` → `162,678 ns/op`, `241,640 B/op`, `4,134 allocs/op`
  - `BenchmarkRunManagerListWorkspaceRuns`: `893,060 ns/op`, `274,764 B/op`, `7,701 allocs/op` → `349,162 ns/op`, `179,525 B/op`, `4,492 allocs/op`
  - `hyperfine` means: `./bin/compozy --help` `10.0 ms` → `7.8 ms`; `--version` `9.9 ms` → `6.4 ms`; `completion bash` `9.2 ms` → `6.7 ms`
- Passed targeted validation and the full repository gate: `make verify`.

Now:

- None.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-18-MEMORY-daemon-perf.md`
- `.compozy/tasks/daemon/{task_16.md,_tasks.md,_techspec.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_16.md}`
- `.compozy/tasks/daemon/adrs/{adr-001.md,adr-002.md,adr-003.md,adr-004.md}`
- `.compozy/tasks/daemon/perf_analysis/{SUMMARY.md,analysis_storage_sqlite.md,analysis_daemon_runtime.md,analysis_run_executor.md,analysis_event_journal.md,analysis_tui_rendering.md,analysis_cli_startup.md}`
- Commands: `go test ./internal/store/rundb ./internal/daemon -run '^$' -bench 'BenchmarkRunDBListEventsFromCursor|BenchmarkRunManagerListWorkspaceRuns' -benchmem -count=1`, `hyperfine --warmup 3 --runs 10 './bin/compozy --help > /dev/null' './bin/compozy --version > /dev/null' './bin/compozy completion bash > /dev/null'`, `make verify`, `git commit -m 'perf: optimize daemon performance hot paths'`, `git status --short`
