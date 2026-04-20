- Goal (incl. success criteria):
- Fix TUI responsiveness regressions so local and remote/daemon-backed cockpit updates remain fluid under streaming load.
- Success means: stream delivery does not stall behind render speed, elapsed time/countdown labels advance in real seconds, lifecycle events are preserved, focused regression tests/benchmarks cover the hot path, and `make verify` passes.

- Constraints/Assumptions:
- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills active this session: `systematic-debugging`, `no-workarounds`, `bubbletea`, `extreme-software-optimization`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` before completion.
- Root-cause fix only: no timing hacks, no weakening tests, no destructive git commands.
- Preserve public CLI/daemon transport contracts unless verification proves an internal-only change is required.

- Key decisions:
- Treat the main regression as architectural coupling between event ingestion and Bubble Tea render/update throughput.
- Replace the intermediate `waitEvent()`-driven queue with controller-owned delivery/coalescing built around Bubble Tea program message injection.
- Separate real-time clock updates from spinner animation so second counters are aligned to wall-clock seconds rather than a generic 120ms UI tick.
- Keep lifecycle/terminal messages lossless; only cumulative UI snapshots/usage deltas may be coalesced.

- State:
- Completed after clean verification.

- Done:
- Reviewed relevant ledgers for previous TUI/daemon attach/perf work.
- Re-read the required skill files and Bubble Tea/Bubbles implementation details for `Program.Send`, `Tick`/`Every`, and `viewport.SetContent`.
- Confirmed current root cause in code:
- `uiModel` drains one `uiMsg` per update cycle via `waitEvent()`.
- `uiController.Enqueue` writes into a parallel buffered channel rather than the Bubble Tea program directly.
- `translateSessionUpdate` emits a full snapshot for every session update.
- `handleTick` uses one 120ms tick for both spinner and elapsed-time refresh.
- Reworked the TUI controller/model pipeline:
- controller now batches/coalesces pending inputs and injects `dispatchBatchMsg` into Bubble Tea via `Program.Send`
- raw lifecycle/session/usage events from the local bus and remote stream now flow through the same controller-owned translator/coalescer
- coalesced updates flush before lifecycle messages so ordering is preserved while cumulative snapshots/usages still collapse per batch
- Split time handling:
- `clockTickMsg` uses `tea.Every(time.Second, ...)` for elapsed/countdown updates
- `spinnerTickMsg` uses `tea.Tick(100*time.Millisecond, ...)` only while jobs are active
- `formatDuration` and shutdown countdown now use truncation/floor semantics based on model time rather than generic render-time rounding
- Added sidebar row caching and content diffing so unchanged rows/content skip expensive viewport refresh work.
- Updated remote attach/follow so live daemon stream items enqueue raw `events.Event` values instead of pretranslated UI messages.
- Added focused regression coverage for:
- raw adapter forwarding and unsubscribe behavior
- dispatch batch coalescing + lifecycle ordering
- exact-second clock behavior / spinner shutdown
- remote attach/follow raw-event delivery
- Added focused benchmarks:
- `BenchmarkPrepareDispatchBatchSessionBurst`: `694783 ns/op`, `472363 B/op`, `11146 allocs/op`
- `BenchmarkRefreshSidebarContentCachedRows`: `2608 ns/op`, `11392 B/op`, `2 allocs/op`
- Passed package-level validation with race detector: `go test ./internal/core/run/ui -race -count=1`
- Passed final repository gate: `make verify`

- Now:
- None.

- Next:
- None.

- Open questions (UNCONFIRMED if needed):
- None.

- Working set (files/ids/commands):
- `.codex/ledger/2026-04-19-MEMORY-tui-realtime-fix.md`
- `.codex/plans/2026-04-19-tui-realtime-fix.md`
- `internal/core/run/ui/{model.go,update.go,types.go,sidebar.go,view.go,remote.go,adapter_test.go,update_test.go,remote_test.go,model_test.go,bench_test.go}`
- `internal/core/run/transcript/model.go`
- Commands: `go test ./internal/core/run/ui -count=1`, `go test ./internal/core/run/ui -race -count=1`, `go test ./internal/core/run/ui -run '^$' -bench 'Benchmark(PrepareDispatchBatchSessionBurst|RefreshSidebarContentCachedRows)$' -benchmem -count=1`, `make verify`
