# Loop runtime and frontend performance audit

Baseline: `de5ab2f4d`. Owned implementation and focused verification are complete; runtime/browser and current-head CI evidence are owned by the enclosing audit.

## Owning invariants and suites

- Complete briefing/progress rosters preserve every node, generation, state, ordering and fan-out rollup independently of the public 500-row page maximum. Owner: `internal/loop` roster projection. Canonical suites: `TestBriefingContract` in `briefing_test.go`, `TestRosterContract` in `roster_test.go`. Measurement: `BenchmarkCompleteRoster` in the existing briefing suite.
- Every accepted Loop stream frame reaches the live consumer immediately; authoritative scoped reads converge after lifecycle bursts without one canceled request per frame, and pending reconciliation survives stream disposal/error. Owner: `web/src/systems/loops/hooks/use-loop-stream.ts`. Canonical suite: `hooks/__tests__/use-loop-stream.test.tsx`, using the existing EventSource I/O fake with real Query observers.
- An accepted Loop event cannot be reconciled solely by a fetch started before the event. Initial reads and prefetches settle before trailing invalidation; disabled observers stay invalidated without forced reads and refresh when enabled. Owner: `lib/loop-live-refresh-coordinator.ts`; canonical suite remains `hooks/__tests__/use-loop-stream.test.tsx`, including disposal and failed-initial-read cases. This closes the preexisting L1 limitation recorded in `review-cache-loops.md`.
- Scheduler wake selection preserves scope, capability, ownership, ordering, occupied-session exclusion and starvation classification. Owner: `internal/scheduler`. Canonical suite: `scheduler_test.go` (existing `TestRunOnceWakesOnlyEligibleIdleSessions`, `TestRunOnceEscalatesStarvedRuns`, ordering/owner/capability cases), with `BenchmarkSelectWakeTargets` measuring cross-workspace scan amplification.
- Scheduler mutations reconcile status and every cached backlog scope after success or failure without canceling a redundant status read. Owner: scheduler mutation hooks. No existing scheduler hook suite owns cache mutation behavior (the controls component suite receives callback props only), so `hooks/__tests__/use-scheduler-actions.test.tsx` is the canonical suite for the three existing hooks. It uses real Query observers and mocks only the API I/O boundary.

## Measured roster improvement

`projectCompleteRoster` rebuilt all roster indexes and projections once for each 500-row page. The complete internal projection now executes once through the same projection used by public pagination. Public query normalization, filtering, lineage ordering, fan-out rollups, cursor validation and pagination remain at their existing boundaries. No persisted data, DTO, SQL query, cache or concurrency was added.

Apple M4 Max, Darwin arm64, Go benchmark CPU limit 2, five samples at 200 ms each; medians below. Fixture: one action with the indicated number of succeeded item outputs, all returned and counted every iteration. This measures projection cost and allocations, not end-to-end HTTP or browser latency.

| Items | Before time | After time | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 500 | 354,315 ns | 326,359 ns | 423,942 | 333,821 | 6,277 | 6,276 |
| 2,500 | 8,955,117 ns | 1,818,786 ns | 9,867,758 | 1,648,820 | 171,516 | 34,296 |
| 10,000 | 141,978,521 ns | 8,795,396 ns | 142,719,552 | 6,719,757 | 2,787,240 | 139,353 |

At 10,000 items: 93.8% less elapsed projection time, 95.3% fewer bytes and 95.0% fewer allocations. The one-page negative control also improves slightly. Raw receipts: `.cache/performance/loop-loops/roster-before.txt` and `roster-after.txt`.

Identical before/after command:

```sh
python3 .cache/performance/run-bounded.py go test ./internal/loop -run '^$' -bench '^BenchmarkCompleteRoster$' -benchtime=200ms -count=5
```

## Measured stream reconciliation improvement

`useLoopStream` previously invalidated four to seven query prefixes per lifecycle frame. It now follows the existing Tasks microtask/in-flight pattern: immediate frame delivery, one active read round and one pending union of affected scopes. The union contains at most four scope labels. No timers, persistent cache, event payload buffer or cross-system helper were added. Run/catalog/request/goal-turn scopes retain the canonical key factories; overlapping run-read and goal-turn prefixes are reconciled once.

Same delayed-I/O fixture before and after, pinned Bun 1.4.0 through the bounded runner: 100 lifecycle events at 2 ms intervals, a 100 ms abort-aware read, a subscribed real Query observer, and an unrelated run cache. Read starts fall **100 → 3 (97% fewer)**; cancellations **99 → 0**. Every frame is delivered and the authoritative final revision is 100. Counts describe request amplification, not measured browser interaction latency. Receipts: `stream-before-pinned.txt`, `stream-after.txt`.

The complete 28-test canonical hook suite passes, including scheduled/in-flight disposal, stream error, initial-read freshness, failed first read followed by a successful read, late replaced-subscription rejection, listener cleanup, display-only events and callback identity stability. Cleanup prevents new events while letting previously accepted work reconcile; inactive caches remain stale for the next observer. The follow-up waits for the active read round to settle. Coalescing is subscription-local: simultaneous streams can still compete over shared aggregate lists; no global coordination claim is made.

```sh
python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx --maxWorkers=1
```

### Review follow-up L1: initial reads cannot erase an accepted wake

The independent `review-cache-loops.md` reproduction found a preexisting limitation in both the old direct invalidation and the first coordinator implementation: TanStack Query reuses an in-flight initial fetch when no data exists. Its older result can clear the event's invalidation without a post-event read. A disabled observer with an active prefetch can lose the invalidation the same way.

Each affected query prefix now captures its currently fetching promises, waits with `Promise.allSettled`, then invalidates through the existing active-only policy. This follows the session read reconciler's existing wait-before-invalidate boundary. Rejected initial reads still lead to the trailing refresh. Disabled observers are left invalidated without a forced request; enabling them performs the fresh read. No subscription-generation checks, cleanup transitions, key scopes, timers or other production sources changed in this follow-up.

The same five new canonical cases ran before and after the fix. Before: four failed (active initial read, disposal during it, disabled observer with prefetch, rejected initial read), while the disabled-without-prefetch control passed. After: all five pass within the complete 28-test suite. Active/disposed/rejected cases finish at revision 1 with two reads and no cancellation. The disabled prefetch remains at one read until enabled, then finishes at revision 1 after its second read; the disabled-without-prefetch case performs zero reads until enabled and one afterward. The 100-frame performance fixture remains at three reads, zero cancellations and final revision 100.

Receipts: `.cache/performance/loop-loops/stream-initial-negative.txt`, `stream-initial-after.txt`; focused formatting/lint passed in `stream-initial-lint.txt`. The negative control used the same root Turbo command with `--testNamePattern='Should reconcile a wake across'`; the final run used the complete canonical suite command above. Independent reviewer revalidation passed all eight conditions, including disabled prefetch, cleanup without an observer, disposal, initial rejection and A1→B→A2 late-event admission (`.cache/performance/review-cache-loops/recheck-results.txt`, Bun 1.4.0). Production was frozen after formatting at `2026-11-02T03:18:25.339770Z`; root owns the final candidate rebuild and browser/CI verification.

## Measured scheduler mutation improvement

Settlement previously invalidated status and then the parent scheduler key, immediately canceling/restarting the same status read. It now invalidates the parent once. Across pause, resume and drain, each with successful and failed mutation I/O: **status reads 2 → 1**, **cancellations 1 → 0**, both cached backlog scopes still read exactly once and converge. The unrelated query stays untouched. These six observer fixtures and the existing six controls-panel tests pass. Receipts: `scheduler-actions-before.txt`, `scheduler-actions-after.txt`.

```sh
python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/scheduler/hooks/__tests__/use-scheduler-actions.test.tsx src/systems/scheduler/components/__tests__/scheduler-controls-panel.test.tsx --maxWorkers=1
```

## Scheduler selection measurement

Classification visited every session and allocated an entire candidate slice once per queued run. The change indexes already ordered sessions by normalized workspace for one selection cycle, then preserves all existing capacity, ownership, occupied-session and starvation checks. One local candidate buffer is reused only after the prior disposition is consumed and selected sessions copied into wake targets. It is bounded by the supplied session population, never pooled/shared across sweeps, and introduces no goroutines.

The first indexing-only candidate reduced the 50-workspace fixture from 12.29 ms/42.89 MB to 1.82 ms/2.95 MB, but the single-workspace control had noisy slower timings (33.54 → 38.14 ms). An alternating paired repeat showed substantial shared-host variance and retained the avoidable per-run allocation. Those preliminary receipts are preserved as `scheduler-before.txt`, `scheduler-after.txt`, `scheduler-paired.txt`; they are superseded by the retained implementation below.

Final fixture: 500 queued runs and 500 eligible sessions, either one workspace or 50 evenly populated workspaces. All 500 targets are returned every iteration. Before/after test binaries use the same canonical benchmark, with baseline production selection/classification in an isolated package copy and current production in the owning package. Five alternating 200 ms pairs run under the shared bounded runner; no production files are swapped for measurement. Median results:

| Workspaces | Before time | After time | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 30,767,393 ns | 26,155,312 ns | 42,886,922 | 2,199,382 | 531 | 42 |
| 50 | 12,072,208 ns | 1,197,576 ns | 42,886,904 | 2,268,141 | 531 | 289 |

The 50-workspace fixture improves **90.1% in time and 94.7% in allocated bytes**. The single-workspace control improves **15.0% in time and 94.9% in allocated bytes**; its worst after sample (27.07 ms) remains below its best before sample (27.92 ms). This is scheduler selection cost, not total dispatch or daemon latency. Receipts: `scheduler-buffer-paired.txt`, `scheduler-summary.json`. Final scheduler race verification passes in `scheduler-buffer-tests.txt`.

Canonical benchmark command, using the same fixture for each source version:

```sh
python3 .cache/performance/run-bounded.py go test ./internal/scheduler -run '^$' -bench '^BenchmarkSelectWakeTargets$' -benchtime=200ms -count=5
```

The actual alternating execution is retained in `.cache/performance/loop-loops/scheduler-paired.py`, invoked through the bounded runner and recorded in `scheduler-buffer-paired.txt`.

## Audit coverage

Each package/system was inspected separately. Unchanged entries describe static findings, not measured latency claims or a claim that every path is optimal.

| Package or system | Inspected paths and outcome |
| --- | --- |
| `internal/loop` | `roster.go`, `run_read_briefing.go`, `run_read_service.go`, `coordinator_history.go`, `coordinator_target_health.go`, `generation_output_runtime.go`: complete projection amplification fixed as above. Production roster loading already uses optional bulk output/route-cause reads, with per-generation fallback for other implementations. Runtime payload resolution verifies full workspace/run/generation/node/item/ref authority; deduplication by hash alone would remove that check. Timeline backward reads project accumulated batches until a complete public page boundary; further projection batching is unmeasured. No concurrent SQLite readers added. |
| `internal/loop/dsl` | `graph.go` and parameter materialization: YAML-based decoding preserves arbitrary node parameter semantics. No measured bottleneck justified changing decoding or adding a cache. |
| `internal/loop/dsl/refs` | `template.go`: validated templates retain parsed representations; runtime interpolation parses with curated functions and strict missing-key behavior. No unbounded template cache added. |
| `internal/loop/gate` | `evaluator.go`: ordered criterion evaluation may invoke commands/extensions against shared workspace state. Parallelization requires an independence contract and provider evidence; none introduced. Aggregation remains deterministic. |
| `internal/loop/goal` | `executor.go`, `turns.go`, checkpoint/runtime/binding types: turns and control checkpoints form a serial ownership chain. Segment contexts cancel on exit; parallel turns would alter checkpoint, budget and prompt ownership. No change. |
| `internal/loop/watch` | `adapter.go`: each tick completes one poll/ready/settle/confirm cycle and does not retain a parked goroutine. No measured actionable bottleneck. |
| `internal/task` | `manager_read.go`, `manager_views.go`, `manager_reconcile_status.go`, `catalog_service.go`, `wake_dedupe.go`; traced downstream `global_db_task_catalog.go`, `_sql.go`, `_scan.go`. Public task lists already use authorized batched catalog projections. Legacy enriched `ListTasks` and expanded `GetTask` children still perform per-child status/dependency/run reads. Their richer authorization/status/dependency semantics cannot be replaced blindly by catalog rows; no measured supported replacement retained. Wake dedupe has a bounded 4,096-id FIFO, with eviction copying worth future profiling. No SQL/schema changes made. |
| `internal/scheduler` | `selection.go`, `capacity.go`, `wake_dispatch.go`, `scheduler.go`: workspace indexing and local candidate reuse above. Batch wake dispatch already exists; its serial fallback preserves admission ordering. Scheduler runtime has cancellation/join ownership; expired cooldown entries are pruned. No extra goroutines. |
| `internal/automation` | `list_resource_catalog.go`, `trigger_dispatch.go`, `schedule_lifecycle.go`, `schedule_execution.go`: catalog queries fetch enabled overrides once and hydrate rich records only after filtering/sort/page selection. Trigger dispatch preserves stable trigger order. Job workers have per-job cancellation and runtime join ownership; dispatch admission already bounds active work. No measured supported change. |
| `internal/automation/model` | `list_semantics.go`, `list.go`, `run_cursor.go`, template/validation paths: profile/workspace filters and stable list order/cursor boundaries remain authoritative. No runtime cache or goroutine is owned here; no change. |
| Web `loops` | `hooks/use-loop-stream.ts`, `hooks/use-loop-run-reads.ts`, `lib/query-options.ts`, `lib/loop-run-live-seam.ts`, DAG view/edges helpers: stream invalidation measured separately. Roster auto-loading is bounded to 10 × 200 rows before explicit continuation. Timeline history intentionally unions previously loaded entries, deduplicates coalesced spans, preserves identity and latches a profile/run/workspace-specific stream fence. That retained history can grow while a run stays open; truncating it would drop user-loaded facts. No speculative memoization or global cache. |
| Web `automation` | `lib/query-options.ts`, `hooks/use-automation.ts`, filter/form helpers: infinite catalogs retain page envelopes and population-fenced placeholder data; profile scope stays in query identity. Hidden consumers can disable polling. Run-history polling is 15 seconds; no per-frame stream fan-out found. No measured supported change. |
| Web `scheduler` | `lib/query-options.ts`, `lib/query-keys.ts`, `hooks/use-scheduler-actions.ts`, controls panel: scoped backlog/status reads poll every 30 seconds with a 15-second stale window. Duplicate settlement invalidation measured separately; root invalidation already covers all scheduler keys. |

## Validation and limits

All test/benchmark processes use the shared bounded runner. Frontend focused tests run from the root through Turbo `--only`, as authorized while shared generation work is in progress. The parent audit owns generated assets, full integration/runtime/browser QA, delivery gates and exact-head CI.

The focused Go race run passed `TestBriefingContract`, `TestRosterContract` and the scheduler `TestRunOnce*` cases. The final candidate-buffer implementation also passed the separate scheduler `TestRunOnce*` race run. No SQL/schema/generator changes were made by this owner.

```sh
python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race ./internal/loop ./internal/scheduler -run '^(TestBriefingContract|TestRosterContract|TestRunOnce.*)$' -count=1 -parallel=2
python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race ./internal/scheduler -run '^TestRunOnce.*$' -count=1 -parallel=2
```

Focused Oxlint over the five changed TS/TSX files passed with `--deny-warnings` (`owned-checks.txt`). Go test-convention heuristics report zero findings in the briefing suite and the same 12 preexisting inline-case findings in the scheduler suite as baseline, with zero introduced findings. Existing tests were not renamed or weakened. Final `gofmt`, Oxfmt for the five TS/TSX files, and scoped `git diff --check` completed; root owns the integrated TypeScript check and delivery lint. The optional final report-format invocation found no target because `.oxfmtrc.json` explicitly excludes `docs/**`; no ignore rules were overridden (`final-format.txt`).

Preserved unsuccessful receipts: `stream-before.txt` encountered concurrent sqlc generation drift in another owner's extension query; `stream-before-focused.txt` exposed an unstable test EventSource factory (the factory now stays stable); `roster-tests.txt` exposed missing durable generation lineage in the newly added multigeneration fixture (the fixture now supplies both authoritative generation records). Assertions were retained; production semantics were not relaxed to accommodate those fixtures. `test-conventions.txt` records an initial two-path invocation of a single-file checker; the corrected per-file/baseline comparison is in `owned-checks.txt`.

Affected QA scenarios for the parent integration walk: `LP-web-run-default-read-briefing`, `LP-web-runs-roster-rerank`, `LP-web-loops-all-projects`, `LP-web-pending-request-provenance`, `TA-044`, `TA-046`, `TA-047`, `TA-048`. Native tools, hooks, extensibility, config, persisted workspace data and official Compozy skill contracts retain their existing shapes and behavior; the internal projection also serves their existing reads. No migration or public compatibility shim is needed.

## Changed files

- `internal/loop/roster.go`
- `internal/loop/run_read_briefing.go`
- `internal/loop/briefing_test.go`
- `internal/scheduler/selection.go`
- `internal/scheduler/capacity.go`
- `internal/scheduler/scheduler_test.go`
- `web/src/systems/loops/hooks/use-loop-stream.ts`
- `web/src/systems/loops/lib/loop-live-refresh-coordinator.ts`
- `web/src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx`
- `web/src/systems/scheduler/hooks/use-scheduler-actions.ts`
- `web/src/systems/scheduler/hooks/__tests__/use-scheduler-actions.test.tsx`
- This report and `docs/qa/scenarios/<id>.md` for the eight scenario IDs listed above. Scenario changes append QA impact only and preserve the existing frontmatter/history.

No changes were retained in task, automation, their persistence queries, or the frontend automation system.
