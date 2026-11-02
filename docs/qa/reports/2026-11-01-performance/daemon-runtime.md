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
