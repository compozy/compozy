# Performance Analysis — Cross-Cutting Summary

Six parallel analyses covering daemon runtime, storage/SQLite, run executor, event journal, TUI rendering, and CLI startup. Each agent used the `extreme-software-optimization` skill.

Individual reports:
- [CLI Startup & Dispatch](analysis_cli_startup.md)
- [Daemon Runtime](analysis_daemon_runtime.md)
- [Storage / SQLite](analysis_storage_sqlite.md)
- [Run Executor](analysis_run_executor.md)
- [Event Journal & Streaming](analysis_event_journal.md)
- [TUI Rendering](analysis_tui_rendering.md)

---

## Cross-cutting hotspots (same root, multiple reports)

### X1. `ListEvents` has no SQL `LIMIT` — O(N²) replay
- **Flagged by:** daemon (F2) + journal (P0-1)
- **Where:** `internal/store/rundb/run_db.go:273` consumed by `internal/daemon/run_manager.go:471-527`
- **Impact:** 10k-event run attach allocates ~10 MB/page, full table re-scan per page; replay is O(N²/P)
- **Fix:** push `LIMIT ?` into SQL; drop the redundant in-Go `EventAfterCursor` filter

### X2. Per-request SQLite handle open/close + migration check
- **Flagged by:** daemon (F3) + journal (P0-3)
- **Where:** `internal/daemon/run_manager.go:414, 482, 1549, 1563`
- **Impact:** every `Snapshot`/`Events`/terminal-resolve opens a fresh `rundb.RunDB`, re-parses pragmas, re-checks schema
- **Fix:** per-run LRU handle cache with idle TTL, shared by concurrent readers

### X3. N+1 workflow lookup in `RunManager.List`
- **Flagged by:** daemon (F1) + storage (P0-3)
- **Where:** `run_manager.go:382-391, 1279-1289` → `GlobalDB.GetWorkflow` per row
- **Impact:** 100 runs = 101 SQLite round-trips; contends with writer lock under SQLite single-writer
- **Fix:** `WHERE id IN (...)` batch or `LEFT JOIN workflows` in one query; or denormalize slug onto `runs`

### X4. Redundant JSON (de)serialization on the streaming path
- **Flagged by:** executor (P0-A) + journal (fsync amplification)
- **Where:** `PublicSessionUpdate` → `runtimeevents.NewRuntimeEvent` → journal `json.Encoder.Encode` → stdout streamer
- **Impact:** same envelope marshaled 3–4× per event; 40–60% CPU on streaming
- **Fix:** marshal once, reuse `json.RawMessage`; pass pre-encoded bytes through journal + transport

### X5. Per-frame/per-event dead work regardless of change
- **Flagged by:** TUI (P0-1/2/3) + executor (P1-C) + CLI (P0-2/3)
- **Pattern:** recompute on every tick/event/invocation even when nothing changed or nothing will observe the output
  - TUI: `reapplyOwnedBackground`, `viewport.SetContent` on cache hit, sidebar rebuild every 120 ms
  - Executor: `formatBlockTypes` builds sorted map every log call regardless of log level
  - CLI: dispatcher + event bus + agent registry constructed for `--help`/completion
- **Fix:** dirty flags, `logger.Enabled` gates, `sync.OnceValue` / lazy factories

---

## P0 priority ordering (across all reports)

| # | Finding | File | Estimated win |
|---|---------|------|---------------|
| 1 | `ListEvents` unbounded SQL | run_db.go:273 | Replay O(N²)→O(N); multi-second → ms on large runs |
| 2 | RunDB handle churn | run_manager.go:482,1549 | Eliminate open+migrate per read (~ms per call) |
| 3 | N+1 workflow lookup | run_manager.go:382 | 101 queries → 1 on list |
| 4 | Dead indexes on write-hot path | schema.go events/job_state/transcript | 30–50% less write amplification |
| 5 | `LOWER(TRIM(status))` kills index | runs status predicates | Seeks instead of full scan |
| 6 | Triple-marshal on session.update | executor + journal + streamer | 40–60% CPU on streaming |
| 7 | Observer hook `cloneJSONValue` + goroutine-per-subscriber | extension/dispatcher.go | 3–10× throughput with extensions |
| 8 | fsync per flush on 100ms ticker | journal.go:615 | subscriber latency -3..100 ms/event |
| 9 | Update-check pre-parse | cmd/compozy/main.go:30 | `--help`/completion 250 ms → ~0 |
| 10 | Eager kernel + 40 subcommands | internal/cli/root.go:30,82 | Cold-start cut significantly |
| 11 | `reapplyOwnedBackground` byte-scan | ui/styles.go:146 | 2–6 ms/frame on 120×40 |
| 12 | Timeline cache defeated by `SetContent` | ui/timeline.go:39 | 3–10 ms/frame on long transcripts |

---

## Suggested execution order

1. **Storage wins first** (X1, X2, X3, #4, #5) — highest impact, contained to `internal/store` + `internal/daemon`, blocks other optimizations from being measurable
2. **Streaming marshal collapse** (X4 / #6, #7, #8) — unlocks throughput; needed before extensions/observers become viable under load
3. **TUI frame budget** (#11, #12, X5 TUI half) — user-perceptible, isolated to `internal/core/run/ui`
4. **Cold-start / eager init** (#9, #10, X5 CLI half) — quality-of-life, easy wins, low risk

## Verification strategy

- Add micro-benchmarks under `internal/store/rundb` for `ListEvents` (before/after LIMIT)
- Add `go test -bench` on `runtimeevents.NewRuntimeEvent` + journal append for marshal collapse
- Capture `pprof` CPU + alloc on a real run of a 500-step workflow; compare before/after each P0 fix
- For TUI: Bubble Tea `Update`/`View` micro-bench with a synthetic 10k-event model
- For CLI: `hyperfine 'compozy --help'` and `hyperfine 'compozy completion bash > /dev/null'`
