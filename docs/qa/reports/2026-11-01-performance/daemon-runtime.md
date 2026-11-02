# Daemon, situation, runtime selector and process audit

Baseline: `de5ab2f4d`. Worktree: `perf/responsiveness-audit-20261101`.

## Invariants and owning suites

- Scheduler session capability projections retain agent/workspace resolution, capability ordering and limit, unknown-on-error semantics and cancellation. The scheduler consumes capabilities only; task/soul/spawn-budget enrichment must not make that read slower or fail it. Owners: situation service and daemon scheduler adapter. Canonical suites: `internal/situation/service_test.go`, `internal/daemon/scheduler_runtime_test.go`; performance suite: `internal/daemon/perf_bench_test.go` with real temporary SQLite.
- Scheduler pending runs retain order and all task/run payloads; directly or ancestrally paused tasks stay ineligible, and pause changes between sweeps are visible. Owner: daemon scheduler task source. Canonical suite: `scheduler_runtime_test.go`; same performance suite with real SQLite.
- Closed runtime selectors retain their current selection and advanced controls; opening projects the latest catalog/favorites with the same browse/search/keyboard semantics. Owner: runtime selector controller. Canonical suite: `web/src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx`.
- Session process polling preserves idle and tool-busy scan cadence and attached-client demand, while owner hydration occurs only for an eligible scan. Owner: sessionprocess loop; canonical suite: `internal/sessionprocess/attribution_test.go`, with the manager scalar projection covered by the session owner's suite.
- Connection and home-directory consumers share the bounded identity request, refresh identity changes, retain error/cancellation behavior, and never trigger full diagnostics; explicit diagnostic consumers preserve the complete status payload and restart-required behavior. Owner: status queries/adapters and workspace home hook. Canonical suites: `web/src/systems/status/hooks/__tests__/use-daemon-health.test.ts`, `web/src/systems/status/adapters/__tests__/daemon-api.test.ts`, and the existing `useUserHomeDir` block in workspace's `use-workspaces.test.tsx`. The OS owner covers closed/open About requests in its existing dialog suite.
- Task audit projections preserve scoped task identity, event ordering/payloads, worktree filtering, empty populations, fresh writes and context failure while avoiding unrelated events. Owners: observe snapshot and globaldb task audit persistence. Canonical suites: `internal/observe/tasks_test.go` and `internal/store/globaldb/global_db_task_graph_audit_test.go`; real SQLite fixture and event digest are measured in `internal/observe/perf_bench_test.go`. No schema/index/migration change is involved.

## Measured changes

All Go measurements used Apple M4 Max, darwin/arm64, `GOMAXPROCS=2`, `GOFLAGS=-p=2`, five repetitions and 200 ms benchmark budgets through the shared bounded runner. Values below are medians; fixture construction and database migration are outside timed loops. These are component-level CPU/allocation measurements, not end-user latency.

| Workload | Before | After | Allocation before → after |
| --- | ---: | ---: | --- |
| Scheduler capabilities for 128 sessions; real SQLite task store, empty task population | 9.672 ms/op | 0.0387 ms/op | 1,212,717 → 21,632 B/op; 25,475 → 2 allocs/op |
| Scheduler task projection, 64 distinct tasks × four runs | 27.700 ms/op | 4.787 ms/op | 2,832,878 → 1,004,178 B/op; 63,024 → 12,316 allocs/op |
| Scoped observe task snapshot, one of 64 tasks × 16 events | 4.367 ms/op | 0.304 ms/op | 1,549,927 → 38,713 B/op; 26,976 → 760 allocs/op |

The capability adapter previously constructed full context including unrelated soul, spawn-budget and task context. It now resolves the same live agent/workspace/skills capability section directly. In this fixture it eliminates 256 unrelated task/run queries per sweep; failures in those unrelated dependencies cannot make capabilities unknown. Capability errors/cancellation still retain the scheduler's existing uncertainty semantics.

Task projection now performs one task lookup and one effective-pause lookup per distinct task in a sweep, preserving run order and rereading pause state in the next sweep. In the fixture this changes 768 SQL reads to 128 (task reads 256 → 64; effective-pause reads 512 → 64). No long-lived cache or concurrency was added.

The process loop still checks demand every 250 ms but only hydrates full owners when a scan is due. `session.Manager.ProcessActivity`, owned and measured by the session worker, reads scalar activity rather than cloning full Info/capability lists. Idle live sessions retain the five-second scan cadence, busy live sessions retain 250 ms scans and attached clients retain their normal cadence. The canonical demand-loop test observes zero owner reads for inactive and idle-before-due cases, and one for the first busy tick, then cancels the loop. The session owner measured 256 idle active sessions with 128 model options: full List plus demand inspection 901,974 → 4,986 ns/op, 3,179,008 → 2,304 B/op and 1,538 → one allocation (five 200 ms repetitions). That baseline already includes the separate redundant-Info-clone fix, so this is the additional scalar-projection improvement. Receipts: `.cache/performance/session-os/process-before.txt` and `session-final.txt`.

Frontend runtime evidence uses the checked-in `web/e2e/scripts/benchmark-runtime-selector.tsx`: 40 fresh SSR mounts with 2,500 curated OpenCode models, three warmups and 20 measured samples. The final harness renders the row count in a `data-rows` attribute and parses it outside the timed loop, avoiding mutation during render. Identical harness bytes were used in both checkouts, with pinned Bun 1.4.0 and consecutive runs under one validation lock.

| SSR diagnostic | Frozen baseline | Candidate | Projected popup rows before → after |
| --- | ---: | ---: | ---: |
| 40 closed selector mounts, 2,500 curated models | 143.728 ms median (133.992–163.073 ms range) | 11.440 ms median (10.791–13.105 ms range) | 100,000 → 0 |

These final results supersede the previous `runtime-paired.txt` measurement. The final receipt is `runtime-paired-final.txt`; it records harness SHA-256 `a77333a52889c74fe8ccad1290673cb7c8f4988178fc0678af40843a60bf43db`, runtime versions and source hashes. The wrapper verified the frozen selector against `de5ab2f4d` and asserted the expected row counts. The controller still computes current selection/advanced controls, and the component suite verifies catalog changes while closed appear on reopening. React SSR includes neither browser rendering nor React Compiler; no browser-latency claim is made from this diagnostic.

Connection/home consumers now share the existing `/api/status/identity` endpoint. The query test mounts two connection consumers, one identity consumer and disabled diagnostics: one initial identity read, zero full status reads, fresh identity on the next ten-second refresh and an error state after failed refresh/retry while retaining the last identity. This preserves full status for Home System, settings restart and open About. The OS owner measured closed About as four → zero full status requests over 30 seconds (baseline at 0/10/20/30 s). Opening then produces one request; ten seconds open produces a second; closing and advancing another 30 seconds keeps the total at two. Its existing pending/resolved field coverage and OS suites pass (79 tests in three files). Receipts and full details are in the session/OS report.

The scoped observe benchmark uses 64 tasks × 16 audit events and a query selecting one task. Baseline: 4.367 ms/op, 1,549,927 B/op, 26,976 allocs/op, decoding 1,024 events before returning 16. After: 0.304 ms/op, 38,713 B/op, 760 allocs/op, decoding only those 16 events. Every repetition before/after produced the same event JSON SHA-256: `83ce00f5e8e75187a3e9734a232a41fd88163e8ff0ec8b6bfdf772200e520164`. The candidate SQL restricts event reads to the already scoped task IDs before decoding; it uses existing task-event indexes and retains descending timestamp/ID order. No schema or migration change is required. Root-owned SQL generation completed. EXPLAIN QUERY PLAN against a fresh real GlobalDB using the checked-in SQL shows the old unrestricted branch scanning `task_events`, versus the new task-ID seek through `idx_task_events_task_seq`; only selected rows need the ordering temporary B-tree. The method remains uncapped for the selected task population to preserve aggregate history semantics; it does not cache across scopes or requests. Focused integration/race checks passed, including task/profile isolation, payload/order/fresh-write/cancellation behavior, and the existing task lifecycle integration journey.

## Concurrency decisions

No new goroutine was justified or added. Removing redundant work reduces SQLite/CPU contention without moving it to another queue. Model-catalog refresh already has bounded parallelism (4), owned admission and shutdown; lifecycle observers preserve required ordering. The measured gains come from reducing work before scheduling it.

## Separately audited packages and systems

| Package/system | Inspected responsibilities | Result and concurrency decision |
| --- | --- | --- |
| `internal/daemon` | Scheduler capability/task sources, session-process bridge, model catalog refresh, worker admission/shutdown, provider-profile events, task role activation, hook fanout, task status replay, mentioned-session history, native query bridges | Remove redundant projection/SQL work. Model refresh already caps background requests at four, coalesces overlapping refreshes and drains owned work. Profile events use a one-item coalescing channel. Mention history stops at 2,000 events or 4 MiB. Existing event activations own shutdown admission; adding per-session goroutines would increase work without removing the measured bottlenecks. Worktree session-guard batching in `boot_worktrees.go` is delegated to the worktree owner. |
| `internal/situation` | Workspace/agent resolution, capability section, full session/startup context, task reference/lease history and rendering | Scheduler now uses a narrow capability projection; full prompt/context behavior stays intact. No cache or asynchronous work introduced. |
| `internal/coordinator` | Bootstrap eligibility, restricted tool allowlist, prompt assembly, run metadata | Pure bounded transformations; no polling or lock. Seven-item allowlist copies are too small to justify a new optimization or concurrency. |
| `internal/heartbeat` | Wake/WakeMany decision path, persistence, authoring revision/rollback | Wake mutex serializes eligibility, cooldown/dedup state and prompt dispatch. Parallelizing these steps would weaken the decision authority and per-cycle wake bound. No measured change justified. |
| `internal/sessionprocess` | Demand loop, serialized process scan, attribution, tree/summaries, capture/termination lifecycle | Read scalar activity before allocating full owners; preserve 250 ms busy and five-second normal cadence. Existing scan semaphore prevents overlapping scans; per-owner goroutines would duplicate host process scans. Summary is an O(1) map read plus port slice copy, so page decoration does not justify batching. |
| `internal/sse` | Scanner, frame/data bounds, handler/error/cancellation semantics | One MiB event/line bounds and reusable data buffer already limit decoding memory. Each emitted payload owns its bytes. Synchronous callbacks preserve stream order/backpressure; no new goroutines or pooling. |
| `internal/monitor` | Admission, durable registration, log/notice queues, retry/stop/recovery | Existing caps (256 total, eight/session, four/terminal), finished retention and coalesced notices bound state. Registration lock includes durability intentionally; publishing before persistence would change admission semantics. No supported change. |
| `internal/observe` | Task summary/dashboard/metrics, scoped task/run reads and audit events; observer persistence/reconciliation; overview fanout; actor-scoped inbox | Scoped dashboard snapshots decode unrelated task audit history before filtering: measure and narrow this read. Overview already uses seven fixed independent errgroup branches with cancellation and distinct outputs; no extra fanout justified. Inbox is bounded and actor-scoped. |
| `internal/observe/attention` | Occurrence/snapshot identity and receipt scope | Copies, sorts and compacts occurrence IDs so snapshots can deduplicate unchanged polling results. Ownership scope remains part of the digest. No independent worker/cache needed. |
| `internal/diagnostics` | Item construction, redaction wrappers, bounded diagnostic output | Delegates secret matching to the redaction owner. Redacting before truncating is required to avoid revealing partial secrets. No separate optimization. |
| `internal/doctor` | Runner, deadlines and host/memory/session/dead-entity/subprocess probes | Sequential deterministic probes have five-second defaults and a 30-second run budget. One buffered result goroutine enforces caller timeout; built-in probes are bounded/context-aware. A noncooperative probe can outlive its timeout, so the wrapper is not a universal worker bound. No speculative parallelism added to this explicit diagnostic path. |
| `web/src/systems/runtime` | Selector controller/list ranking, favorites/recents store, popup keyboard/focus, advanced controls, public exports/mocks/stories | Build popup rows only while open. Selected-model/advanced-state derivation remains live while closed; no new network owner or manual React memoization. |
| `web/src/systems/status` | Typed adapters, query keys/options, health/connection/daemon projections, fixtures and MSW | Connection-only consumers use existing bounded identity endpoint with the same refresh/retry freshness policy; full status remains available unchanged to diagnostics/restart consumers. No parallel poll manager or wire change. |
| `web/src/systems/workspace` (delegated hook only) | `use-user-home-dir.ts` and canonical hook tests | Home-directory resolution follows identity refreshes instead of enabling full diagnostics. The workspace owner retains the rest of the system audit. |
| `cmd/compozy/main.go` | Signal context and CLI dispatch | Thin composition entrypoint, no repeated data loading. Existing signal ownership is appropriate. |
| root `main.go` | `go install` CLI composition | Thin context/stdio dispatcher. No session-proportional or recurring work; no performance edit. |

No nested Go packages exist within the assigned roots except `internal/observe/attention`, listed separately.

## Impact boundaries

Public CLI/HTTP/UDS/native-tool payloads, hooks/configuration and persisted user/workspace data are unchanged.
The scheduler and observe improvements are internal read projections; profile/workspace selection remains
owned by the same canonical query paths. The frontend reuses an existing identity operation and keeps full
status diagnostics/restart semantics. No `skills/compozy/` contract change is needed. QA impact notes are in
`RT-model-catalog-cold-open`, `ET-web-runtime-selector-minimal-slider`, `RT-web-exact-model-id-entry`,
`ET-web-shell-shortcuts-about-dialogs` and `RT-039`; focused checks do not replace the root-owned runtime QA tail.

## Focused validation and reproduction

- PASS: `python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race ./internal/daemon ./internal/situation ./internal/sessionprocess -run 'Test(SchedulerSessionSourcePreservesCapabilityCertainty|SchedulerTaskProjection|CapabilitiesForSession|ProcessScanDemand|DurableLineage|ProcessCatalogChanges|ContextForSessionDependencyContextErrorsContract)' -count=1` (5.587 s daemon, 1.747 s situation, 5.882 s sessionprocess).
- PASS: `python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race -tags=integration ./internal/observe ./internal/store/globaldb -run '^Test(QueryTask|TaskDashboard|TaskHealth|HealthLoadsTaskDataOncePerSnapshot|ObserveTaskLifecycleSummaryAndMetrics|GlobalDBTaskEvent)' -count=1` (real SQLite and task lifecycle integration).
- PASS: root Turbo focused frontend command below: four files, 137 tests (110 runtime selector, 15 workspace, six status hooks, six status adapter); 19.274 s wall time. The dashboard owner additionally passed its nine files/61 tests after migrating connection consumption.
- Scheduler measurement: `python3 .cache/performance/run-bounded.py go test ./internal/daemon -run '^$' -bench '^BenchmarkScheduler(SessionCapabilities|TaskReadProjection)$' -benchtime=200ms -count=5`.
- Observe measurement: `python3 .cache/performance/run-bounded.py go test ./internal/observe -run '^$' -bench '^BenchmarkTaskSnapshotSQLite/FilteredEvents$' -benchtime=200ms -count=5`.
- Frontend command: `python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/status/adapters/__tests__/daemon-api.test.ts src/systems/status/hooks/__tests__/use-daemon-health.test.ts src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx src/systems/workspace/hooks/__tests__/use-workspaces.test.tsx --maxWorkers=1`.
- Runtime diagnostic: `python3 .cache/performance/run-bounded.py bun web/e2e/scripts/benchmark-runtime-selector.tsx`. To compare source revisions, copy only the harness to the frozen checkout and execute it there with that checkout's React dependencies. A discarded cross-checkout import attempt loaded two React copies and was not used as evidence.
- Raw receipts: `.cache/performance/daemon-runtime/{scheduler-before,scheduler-after,focused-race,frontend-tests,runtime-paired-final,observe-before,observe-after,observe-integration-race,observe-query-plan}.txt`.

No daemon, real provider, browser or production user state was started/touched by this worker. Root owns integrated lint/typecheck, generated-file checks, concurrent-session browser/daemon QA, final delivery gates and current-head CI. Static inspection alone is the evidence for packages without a measured implementation change; no synthetic improvement is claimed for them.


## Final change map

- Daemon: `scheduler_session_source.go`, `scheduler_task_source_runs.go`, `session_process_bridge.go`; existing scheduler regression/performance suites.
- Situation: `service_context.go` and its existing service suite.
- Sessionprocess: `service.go`, `types.go` and existing attribution/Unix integration suites. `session.Manager.ProcessActivity` and its measurements are owned by the session worker.
- Observe/store: `tasks_snapshot.go`, existing task/performance suites, `global_db_task_events.go`, existing graph/audit suite, `queries/task_aux.sql`; `sqlcgen/task_aux.sql.go` was generated by root.
- Web: status adapter/query/hook/fixture exports and canonical suites; runtime selector controller and existing component suite; delegated workspace home-directory hook/tests; reproducible runtime benchmark script.
- Coordinated consumers: dashboard connection hook migration is owned by the dashboard worker; OS dashboard/About migration and request-count proof are owned by the session/OS worker. Worktree guard batching is owned by the worktree worker, including its delegated daemon bridge.

No commits or broad gates were run by this worker. All required scoped implementation, measurements and tests above are complete. Root-owned integrated/browser/delivery evidence remains separate from this worker report.

## Additional Go package audit: Web E2E seeders

This read-only inventory follow-up inspected every source file in each of the six packages below. Each package contains only the listed `main.go`; no nested Go package or test file was present. This is direct source inspection, not a coverage inference from test discovery or a passing browser journey. No seeder was executed, benchmarked, changed, or counted as an application performance improvement in this follow-up.

| Exact Go package path | Source read in full | Fixture role | Performance and concurrency disposition |
| --- | --- | --- | --- |
| `web/e2e/fixtures/bundled-extension-seeder` | `web/e2e/fixtures/bundled-extension-seeder/main.go` | Requires an explicit isolated home and source directory, opens its GlobalDB, loads the extension manifest, computes a directory checksum, and installs the local managed extension with bundled provenance. | One synchronous setup operation. Manifest/checksum/install work scales with the supplied extension directory, with DB open/migration and installation delegated to their existing owners. No package-owned goroutine, ticker, retry loop, or repeated session read. Preserve sequential checksum/install ordering; no supported application hot-path optimization or extra parallelism identified. |
| `web/e2e/fixtures/retained-loop-task-seeder` | `web/e2e/fixtures/retained-loop-task-seeder/main.go` | Creates one completed workspace/profile task and one completed task run carrying `loop_run_id`, without creating the retained Loop owner row. This supplies the E2E-020 retention boundary. | Fixed-size fixture writes after one DB open: `CreateTask`, then one parameterized run insert. It does not enumerate existing tasks or run a scheduler. Writes are synchronous and context-bearing; no worker fanout or polling. Preserve task-before-run dependency and keep setup costs separate from product read performance. |
| `web/e2e/fixtures/retired-knowledge-window-seeder` | `web/e2e/fixtures/retired-knowledge-window-seeder/main.go` | Opens isolated GlobalDB/client state, reads the profile's stored window snapshot, adds a retired Knowledge window beside an existing session window, advances and validates topology, writes with `IfRev`, rereads, and prints the stored snapshot. | Full snapshot decode/encode, desktop scan, validation, and reread are proportional to the supplied snapshot; the configured client-state limit bounds serialized values at 16 MiB, not total process memory. Workspace-generation resolution and revision-checked writes preserve the owning boundaries. The caller stops the isolated daemon first to release the single writer. No package-owned goroutines or recurring work; parallel mutation would not be appropriate for this fixture's revision dependency. |
| `web/e2e/fixtures/runtime-database-seeder` | `web/e2e/fixtures/runtime-database-seeder/main.go` | Creates a GlobalDB seed through `storeseed.NewGlobal`, clones it to the required `--output` path, and closes the seed. | One synchronous template creation/clone; no session loop or background work in this command. The inspected TS caller already coalesces template creation with a promise per repository root, supports a prebuilt binary, and copies the closed template to each home with exclusive destination creation. Retain that separation; fixture startup savings would not establish an application responsiveness gain. |
| `web/e2e/fixtures/session-pull-request-seeder` | `web/e2e/fixtures/session-pull-request-seeder/main.go` | Records an existing session's settled branch and a freshly timestamped cached PR/check/review record for topbar PR-chip journeys, then prints the session ID. The TS caller stops and restarts the isolated daemon around the writes. | Two sequential store operations after DB open, with a small supplied failed-check-name list. No forge network request, existing-session scan, polling, or package-owned goroutine. Fresh cached facts are fixture inputs; they are not evidence about live forge refresh latency. Preserve branch-before-PR setup and avoid parallel writers to the same fixture home. |
| `web/e2e/fixtures/sessions-board-seeder` | `web/e2e/fixtures/sessions-board-seeder/main.go` | Creates requested stopped/archived sessions with both catalog rows and on-disk metadata for boot reconciliation; optionally creates one parent and three children, including one failed child, using the normal archive/failure owners. | Linear setup work and returned-ID memory in the requested row count: one registration plus metadata write per row, an archive update per archived row, and fixed family work. Counts are caller-controlled without an explicit CLI maximum. The TS caller stops the isolated daemon first. Sequential writes preserve catalog/meta and family dependencies; there is no package-owned goroutine or recurring scan to optimize. No fixture-throughput result is reported as a board interaction gain. |

Supporting call-path files also read in full: `web/e2e/fixtures/runtime-database.ts`, `web/e2e/fixtures/retained-loop-task.ts`, `web/e2e/fixtures/retired-knowledge-window.ts`, `web/e2e/fixtures/session-pull-request.ts`, and `web/e2e/fixtures/sessions-board.ts`. Those reads establish template reuse, launch-mode isolation checks, and stop/seed/restart sequencing where present; they do not extend this six-package audit to all dependencies.

All six entrypoints use `context.Background()` and join deferred store/seed close errors. No package introduces its own goroutine or deadline. The inspected TS seeding subprocess calls do not specify a per-process timeout, so this audit does not claim a wall-clock cancellation bound. Database APIs receive the command context where available. No source change or additional goroutine is justified by these one-shot setup paths. Runtime evidence for seeders exercised by the root-owned browser lab remains in that lab's receipts; no new test, compile, benchmark, daemon, or browser run was performed for this inventory follow-up.

## CI follow-up: selector popup readiness

The frontend job for `14730a705771c0a3eb6e92947eddede632dff666` failed the existing OpenCode provider-label visibility assertion while 8,377 other tests passed ([run 41383826440, job 125080272563](https://github.com/compozy/compozy/actions/runs/41383826440/job/125080272563)). The invariant remains that an opened selector displays the provider label, exposes the correct accessible option name, and selects the original model ID. Its owning layer and canonical suite are the runtime selector component integration tests, specifically `RuntimeSelector single-line row > Should show the OpenCode model provider after its name without changing its selected ID`.

Cause: `openSelector` returned after finding the popup in the DOM, before it was necessarily visible. The shared `PopoverContent` mounts with `opacity: 0`. The installed Motion 14.6.0 implementation applies the final keyframe through `frame.update` even when `UIProvider` supplies `skipAnimations`; skipping interpolation does not make that write synchronous. The label itself has no hiding style. The original case passed in an unmodified focused local run, so a controlled frame-scheduling probe was used to distinguish readiness from the lazy list projection.

The probe preserved production code and the existing label/accessible-name/selected-ID assertions. A temporary Vitest setup delayed `requestAnimationFrame` callbacks by 250 ms before Motion was imported and observed the row's hidden ancestors. Baseline and candidate used the same probe and environment:

| Source and helper | Controlled frame result |
| --- | --- |
| Frozen `de5ab2f4d`, original presence-only helper | The original label visibility assertion failed; row present, `popover-content` opacity `0`, visibility `visible`, display `block`. |
| Candidate, original presence-only helper | The identical assertion failed with the same hidden ancestor. |
| Frozen `de5ab2f4d`, helper waiting for visible popup | Passed all original assertions after the observer recorded no hidden ancestors. |
| Candidate, helper waiting for visible popup | Passed all original assertions after the observer recorded no hidden ancestors. |

The retained change is confined to the existing `openSelector` helper in `web/src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx`: after finding the popup, it waits for `expect(popup).toBeVisible()` before returning. This strengthens the helper's open-state postcondition. Existing assertions and default timeout budgets remain intact; no retry flag, fixed delay, animation change, or production change was retained. The frozen baseline test was restored to its original bytes after the diagnostic. The 250 ms delay exists only in the ignored diagnostic setup, not the canonical suite.

Final focused validation passed both runtime suites: **114 tests in two files** (110 component tests and four store tests), 14.782 s Turbo wall time. Command: `python3 .cache/performance/run-bounded.py bun x turbo run test --filter=./web --only --force -- src/systems/runtime/components/runtime-selector/__tests__/runtime-selector.test.tsx src/systems/runtime/components/runtime-selector/__tests__/runtime-selector-store.test.ts --maxWorkers=1`.

Receipts under `.cache/performance/daemon-runtime/`:

- `runtime-selector-ci-initial.txt`: original focused case passed before the helper change.
- `runtime-selector-frame-probe-before-mts.txt`: baseline/candidate controlled failures and hidden-ancestor observations.
- `runtime-selector-frame-probe-after.txt`: baseline/candidate controlled passes and baseline-test restoration.
- `runtime-selector-ci-focused-after.txt`: complete 114-test focused validation.
- `runtime-selector-probe.config.mts` and `runtime-frame-delay.ts`: the temporary diagnostic configuration and scheduling boundary. The earlier `.ts` configuration failed ESM loading before tests ran; its `runtime-selector-frame-probe-before.txt` receipt is retained but excluded from the causal evidence.

The paired wrapper selected Bun 1.4.0, but Turbo's task log reported the inner Bun executable at `/opt/homebrew/bin/bun`; the paired comparisons used the same environment. The root owner was notified of this toolchain observation. This repair establishes asynchronous test readiness and preserves the lazy-projection performance evidence; it does not claim a new application speedup or replace current-head CI.

## PR #1141 review: specific error evidence and pause authority

### Included audit-row decoding (`PRRT_kwDORy7nkc6yiTi4`)

Accepted. Invariant: an audit row outside the requested profile cannot break the visible metrics, while including the corrupt row must fail at that row's timestamp decoding. Owner: observe task-metrics projection over the real GlobalDB. Canonical suite: `internal/observe/tasks_test.go`, `TestQueryTaskSummaryKeepsProfileReadsIsolated/Should decode audit rows only for visible tasks`.

The existing all-profile assertion accepted any error. It now requires an unwrapped `*time.ParseError` with `Value == "invalid"` and the `observe: list task events for summary:` diagnostic. This follows the actual `taskEventFromGeneratedFields` → `store.ParseTimestamp` error chain; the timestamp parser does not return `task.ErrValidation`. The successful scoped metric and empty-population assertions remain unchanged. The existing store cancellation assertion remains unchanged and was included in the focused run.

A temporary negative control changed only the all-profile request's `OriginKind` to an unsupported value. The old assertion incorrectly passed (`observe-audit-review-control-origin-before.txt`, 4.433 s). The strengthened assertion rejected that unrelated query-validation error at the expected assertion (`observe-audit-review-control-after.txt`). The valid request was restored before final race validation, which passed against the intentionally malformed SQLite audit row. No new test case, mock, or production change was needed.

### Nil session diagnostic (`PRRT_kwDORy7nkc6yiUWj`)

Accepted. Invariant: `CapabilitiesForSession` rejects nil session information with its declared diagnostic before attempting dependency reads, while preserving context cancellation and dependency failures. Owner: situation service; canonical suite: `internal/situation/service_test.go`, `TestCapabilitiesForSession/Should propagate cancellation and dependency failures`.

The nil-session assertion now requires the exact `situation: session info is required` diagnostic. The existing `errors.Is` assertions for cancellation and the failing skill registry remain intact, so the injected dependency deadline error cannot substitute for the nil-input result. The current production guard already supplies the required diagnostic; only the test changed.

### Per-sweep pause sampling (`PRRT_kwDORy7nkc6yiUWg`)

Disposition: retain the production implementation. The local maps in `schedulerTaskSource.joinRunsWithTasks` and `filterPausedRuns` sample each distinct task once per call; they do not survive into the next sweep. `TestSchedulerTaskProjection` verifies a committed ancestor pause is reread on the next call and removes all affected child runs without reordering eligible peers.

The scheduler emits an advisory notification rather than claiming a run. `schedulerWakeMessage` explicitly states that a queued run may be claimable and that the notification does not assign ownership. The prompt admission gate is not a task-pause barrier. A concurrent pause can therefore cross an already selected notification; the old per-run read path also separated reads, selection, and dispatch and did not provide a zero-notification-after-pause guarantee.

The live authority is the subsequent claim: `task.Service.ClaimNextRun` → `claimNextRunSettlement` → `WithLeaseSettlementTransaction` → `claimNextRunWithExecutor`. Its `baseClaimPredicates` rereads direct task pause, global scheduler pause, and recursive ancestor pause from SQLite inside the writer transaction. Pause mutation uses the same immediate writer-transaction mechanism. Both exact-run and next-run selection use those base predicates. A pause committed before the claim transaction therefore prevents ownership acquisition even if a notification used an older sweep snapshot.

This matches [TA-010](../../scenarios/TA-010.md), whose pause contract makes runs claim-ineligible. The existing `TestGlobalDBClaimNextRunRespectsEffectiveTaskPause` and `TestGlobalDBTaskPauseControlsClaimEligibilityAndBacklog` controls passed again, including inherited pause and subsequent resume. No concrete contract violation requiring a production edit was found. The previously measured 768 → 128 SQL-read reduction is retained; no new performance result or notification-linearization guarantee is claimed.

Read-only authority inspection covered `internal/daemon/scheduler_task_source_runs.go`, `scheduler_waker.go`, `scheduler_runtime_test.go`; `internal/scheduler/wake_dispatch.go`; `internal/task/scheduler_controls.go`, `manager_admission.go`, `lease_manager_claim.go`, `lease_manager_settlement.go`; `internal/store/globaldb/global_db_task_pause.go`, `global_db_task_claim.go`, `global_db_task_claim_select.go`, `global_db_task_lease_settlement_tx.go`, `global_db_task_transaction.go`, `global_db_task_claim_test.go`; and the store writer implementation in `internal/store/write.go` and `write_diagnostics.go`.

### Focused validation and freeze

Race-enabled real-SQLite checks passed in observe (5.761 s), situation (2.102 s), daemon (6.187 s), and globaldb (5.602 s):

```sh
python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race \
  ./internal/observe ./internal/situation ./internal/daemon ./internal/store/globaldb \
  -run '^(TestQueryTaskSummaryKeepsProfileReadsIsolated|TestCapabilitiesForSession|TestSchedulerTaskProjection|TestGlobalDBTaskEventsForTasks|TestGlobalDBClaimNextRunRespectsEffectiveTaskPause|TestGlobalDBTaskPauseControlsClaimEligibilityAndBacklog)$' \
  -count=1 -parallel=2
```

The test-conventions checker reports five existing inline suites in `tasks_test.go` and eight in `service_test.go`, outside the edited subtests. An in-memory comparison with each file at HEAD confirmed identical findings before and after; the full-file heuristic is not reported as clean. The edited cases retain the canonical `Should ...` subtest shape and parallelism. Receipt: `.cache/performance/daemon-runtime/review-assertions-validation.txt`.

Initial control/checker invocation mistakes were retained separately and excluded from behavioral evidence: the first control draft used a nonexistent `TaskMetricsQuery.Scope` field, and the first checker command supplied two paths to its single-file CLI. Neither ran the intended tests. The final valid control and validation receipts above are authoritative.

The batch is frozen with changes only to the two existing test files and this report. No production source, new test file, broad gate, commit, or PR reply was changed by this worker. Integrated lint and current-head CI remain root-owned.


## CI follow-up: hosted MCP prompt timeout and retained diagnostics

The runtime E2E job for `56176f8293d0954e9188041bb47f4796e10138ba` failed `TestDaemonE2EHostedMCPProjectsAndCallsNonBootstrapNativeTool/Should_project_and_call_a_non-bootstrap_native_tool_over_hosted_MCP_stdio` after 32.98 seconds with `PromptSession() error = scan SSE stream: context deadline exceeded` ([run 41392366822, job 125106432535](https://github.com/compozy/compozy/actions/runs/41392366822/job/125106432535)). The assertion is the initial ACP `noop` prompt at `daemon_mock_agents_integration_test.go:809`, after session creation/visibility and before the explicit hosted MCP client, `ListTools`, or native tool invocation. Its 30-second context starts before `CreateSession`, after harness startup. The CI receipt contains no retained phase evidence that identifies which startup, prompt persistence, or delivery boundary stalled.

The existing product invariant is that the managed session completes its initial prompt and the hosted MCP projection subsequently discovers/calls the native tool while preserving hook enforcement. Ownership remains the daemon composition and its canonical E2E above. Read-only tracing covered the test/helpers/fixture, runtime-harness build/process/SSE path, ACP prompt transport, session startup and durable prompt delivery/persistence, API stream termination, and hosted bind readiness. The production daemon/session/ACP/MCP paths do not differ between the previously passing `1665fb0c1` and `56176f829`; this does not prove an unrelated cause or rule out an intermittent defect. The acpmock command also does not activate the Claude bridge-only `WaitForBind` startup gate.

The single focused reproduction passed with the original assertion and 30-second inner deadline. It used the machine-wide verification lock followed by the task serializer, `GOMAXPROCS=2`, `GOFLAGS=-p=2`, `CGO_ENABLED=1`, `-race`, and disposable runtime/home state. A Go overlay restored only the two concurrently edited provider-usage files to `56176f829` for this measurement. Both harness builders inherit `GOFLAGS`, so the test, daemon binary, and acpmock driver used that overlay. Effective command, with the overlay/output paths resolved absolutely by the diagnostic wrapper:

```sh
python3 .cache/performance/run-browser.py env CGO_ENABLED=1 \
  GOFLAGS="-p=2 -overlay=$PWD/.cache/performance/daemon-runtime/hosted-mcp-ci-56176/head-overlay.json" \
  go test -race -tags=integration ./internal/daemon \
  -run '^TestDaemonE2EHostedMCPProjectsAndCallsNonBootstrapNativeTool$/^Should_project_and_call_a_non-bootstrap_native_tool_over_hosted_MCP_stdio$' \
  -count=1 -parallel=1 -timeout=5m -v -artifacts \
  -outputdir="$PWD/.cache/performance/daemon-runtime/hosted-mcp-ci-56176/artifacts"
```

Measured durations: **66.206 seconds for the command**, **32.027 seconds for the Go package**, and **25.11 seconds for the named subtest**, including its daemon build. Safe phase fields in the retained process log show ACP startup completing in **451 ms** (`initialize`: 442 ms) and the later hosted MCP bind in **9 ms**. This macOS focused result does not reproduce the Linux CI suite's concurrency/load. The timeout's cause remains **undetermined and not reproduced**; no runtime repair, timeout increase, assertion relaxation, or retry was retained.

A separate diagnostic defect is proven: `NewArtifactCollector` uses Go 1.27 `testing.T.ArtifactDir`, whose default lifetime ends with the test. The CI runner omitted `-artifacts`, so the reported `/tmp/TestDaemonE2E.../004/e2e-...` directory was removed; the uploader's `/tmp/compozy-e2e-*` pattern also did not identify that test-owned path. The retained fix is limited to `magefiles/test.go:runIntegrationSuite` and the runtime uploader in `.github/workflows/ci.yml`:

- Create a unique `compozy-e2e-artifacts-*` directory under `os.TempDir`, propagate creation errors with `%w`, print its path, and pass standard `-artifacts -outputdir` to the existing runner.
- Upload `/tmp/compozy-e2e-artifacts-*/_artifacts` on runtime-lane failure. This retains test evidence without selecting the temporary lane binaries.
- `runE2ELane` is the only caller. Runtime, combined, and nightly Go suites therefore retain evidence consistently; the nightly uploader's existing `/tmp/compozy-e2e-*` remains compatible. Browser-only script suites are unchanged. No new goroutine or runtime state is introduced.

The diagnostic invariant is that the runtime manifest and process log survive the child Go test process and remain under the uploader's artifact prefix. Its owner is the Mage runner/artifact transport. A temporary Go-overlay probe in the existing `magefile_test.go` suite invokes the real `runIntegrationSuite` for only the existing `TestRuntimeHarnessWriteRuntimeManifestIncludesPathsAndTransportMetadata`; it checks retained manifest/log contents after that child exits. Before the fix, the canonical harness test passes but the probe fails with **zero retained manifests** (11.812 seconds command). After the fix, the same probe passes with **one retained manifest and matching process log** (6.136 seconds command). These durations are validation receipts, not a performance comparison. No new tracked test file or config-only assertion was added. The paired probe uses `python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 GOFLAGS="-p=2 -overlay=$PWD/.cache/performance/daemon-runtime/hosted-mcp-retention/probe-overlay.json" go test -race -tags=mage ./magefiles -run '^TestTemporaryRuntimeArtifactRetentionProbe$' -count=1 -timeout=3m -v`.

Focused Mage lint passed with **zero issues** in 19.756 seconds: `python3 .cache/performance/run-bounded.py env COMPOZY_GO_LINT_SCOPES=./magefiles make go-lint`. The bounded path check maps the runner's observed unique directory name to Linux `/tmp`, verifies the runtime and nightly uploader patterns, and confirms the real focused run still has one manifest and one process log after process exit. This is a local pattern/retention check; the GitHub uploader itself awaits the next CI run.

Receipts: `.cache/performance/daemon-runtime/hosted-mcp-ci-56176/focused.log` and its retained `artifacts/`; `.cache/performance/daemon-runtime/hosted-mcp-retention/{before,after,mage-lint,path-check}.log`, plus the temporary `probe-overlay.json`/`magefile-retention-probe.go`. The initial run is the only real daemon E2E executed in this follow-up; the before/after retention probe uses the existing manifest-only harness test and starts no daemon.

This follow-up changes diagnostic retention only, so native-tool/public/config/Web behavior and QA scenario verdicts remain unchanged. The applicable product journey is `ET-compozy-native-tool-invocation`; the focused result is not a replacement for its full cross-surface verification. Current-head CI remains the acceptance boundary, and a repeat timeout must be diagnosed from the newly retained artifacts rather than labeled fixed by this change.
