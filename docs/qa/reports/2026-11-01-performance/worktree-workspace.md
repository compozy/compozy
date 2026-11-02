# Worktree and workspace responsiveness audit

Baseline: `de5ab2f4d`. Owner: `worktree_workspace`. Focused implementation and verification are complete; integrated delivery remains root-owned.

## Test ownership before implementation

- Worktree listing invariant: every authorized row retains its cached status, exact workspace/repository/branch PR facts, activity and order; repeated rows may share one request-local workspace lookup and one PR cache read. Owner: worktree inspection service plus real global SQLite repository. Canonical suite: `internal/store/globaldb/global_db_worktree_test.go`; existing `internal/worktree/pull_requests_test.go` retains ownership of cache freshness and refresh scheduling.
- Clean status invariant: an observed clean porcelain snapshot reports zero changed files/insertions/deletions and the same branch/upstream/ahead facts without redundant diff subprocesses. Owner: worktree Git status assembly. Canonical suites: `internal/worktree/status_test.go`, existing real-Git lifecycle integration suite.
- Catalog stream invariant: authorized bursts eventually reread all affected exact keys within the existing 150 ms quiet/1 s maximum policy; errors and disconnect flush outstanding work; failure text remains immediate and unauthorized frames remain inert. Owner: workspace catalog-stream hook. Canonical suite: `web/src/systems/workspace/hooks/__tests__/use-worktree-catalog-stream.test.tsx`.
- Detail stream invariant: one worktree wake rereads its list, detail, status and exit plan once while sibling details remain unchanged. Owner: worktree exit-query invalidation; canonical suite: `web/src/systems/workspace/hooks/__tests__/use-worktree-stream.test.tsx`, using real TanStack Query observers.

## Initial findings

- `ListAggregate` and `ListDetails` hydrate each row through `inspectItem`: one status SQL query, one active-session query, one workspace resolution and one PR SQL read per row.
- Workspace resolution can reload workspace/profile config. PR SQL already accepts batch identities, but these callers pass one key at a time.
- `collectStatus` starts both unstaged/staged `git diff --numstat` processes even when the preceding porcelain snapshot is clean.
- Detailed worktree stream already coalesces wake signals; catalog stream immediately invalidates list, aggregate and detail keys for every frame.

## Scope

Initial: `internal/worktree/**`, `web/src/systems/workspace/**`.
Delegated store edit permission: `internal/store/globaldb/global_db_worktree*`; daemon bridge only if batching is measured: `internal/daemon/boot_worktrees.go`.
Follow-up audit: `internal/workspace`, `workspaceaccess`, `workspacefiles`, `workref`, `windowmanager`, `clientstate`, `support`, `logger`, `retry`, `demoseed`; frontend `theme`, `design-system`. All eleven assigned Go packages are flat; no nested Go package was omitted. The inspected path inventory is retained in `.cache/performance/worktree-workspace/audited-paths.txt`.

## Per-package audit ledger

| Package/system | Inspected paths and behavior | Disposition |
| --- | --- | --- |
| `internal/worktree` (flat package) | `inspection.go`, `aggregate.go`, `status*.go`, `discovery.go`, `git_metadata.go`, `pull_request*.go`, catalog/event broadcaster, creation/exit lifecycle and existing suites | Measure listing hydration, clean Git refresh; preserve bounded PR refresh scheduler (four workers), cancellation and repository locks. |
| `web/src/systems/workspace` | query factories/adapters, `use-worktrees`, catalog/detail streams, exit ladder/context, workspace tree projections and existing hook tests | Measure catalog wake amplification; existing detail stream already coalesces. |
| `internal/workspace` (flat package) | resolver, snapshot freshness, CRUD/list reconciliation, clones, cache eviction and `perf_bench_test.go` | Registration-only and short-lived content-checked runtime caches already exist; request-local batching at the worktree caller removes redundant full resolution without weakening fresh config/root checks. No additional cache. |
| `internal/workspaceaccess` (flat package) | `default_policy.go`, request facts/consent decision chain | Same-workspace access already avoids audit writes; operator fast path and one current session-facts lookup retain authorization truth. Caching permission decisions is inappropriate. No change. |
| `internal/workspacefiles` (flat package) | `index.go`, `collect.go`, `rank.go`, canonical index tests/benchmarks | Root-specific in-flight build sharing, bounded build deadline, 25,000-file/100,000-walk caps, eviction, precomputed empty-query order and bounded top-k ranking already exist. No measured additional bottleneck; do not spawn parallel walkers. |
| `internal/workref` (flat package) | reference value objects and normalization | Small immutable value/normalization helpers; no repeated I/O, retained state or scheduling owner. No change. |
| `internal/windowmanager` (flat package) | service execute/load/normalize, client context/projections, snapshot clones, active coalescer, subscriptions/repository and lifecycle | Existing active-tab persistence coalesces for two seconds; workspace locks serialize authoritative transitions. Snapshots/history require isolation. Deep cloning remains a profiling candidate, but changing its ownership without proof is unsafe; no retained change. |
| `internal/clientstate` (flat package) | engine/service, bbolt apply/count, sequencer, hub/subscription, clones/limits | Per-workspace sequencing, bounded 256-event buffers, slow-consumer eviction and 512-key quota are already explicit. Avoid concurrent mutation execution that would break sequence fences. No change. |
| `internal/support` (flat package) | service/builder/lifecycle, archive writer, tail reader and home-tree collection | Operator-triggered asynchronous work has owner cancellation/join, byte limits and retention. Not in ordinary session navigation. Directory walking after its entry limit could be stopped sooner but is not retained without a relevant measurement. |
| `internal/logger` (flat package) | logger, redacting handler, failure warning summaries | Constant-space failure fingerprint plus five bounded samples and repeated-warning backoff already protect recurring failures. Redaction remains mandatory; no speculative bypass. |
| `internal/retry` (flat package) | retry policy/backoff and context-aware wait | Attempts/delays bounded; cancellation checked before and after I/O; timer owned by caller. No additional goroutine or caching needed. |
| `internal/demoseed` (flat package) | seed orchestration/content/fixture files | Explicit offline fixture generation with dependent writes, fixed scenarios and bounded cleanup. No production navigation hot path; parallel SQLite writes would not improve responsiveness. No change. |
| `web/src/systems/theme` | singleton runtime/store selectors, font preference sync, lazy bundled-font and installed-font loading | Theme writes only on selected changes; subscriptions clean up. Installed fonts queried once after picker gesture; no session-stream dependency. No change. |
| `web/src/systems/design-system` | two Storybook-only showcase route/component files | No production data loading or retained runtime state. No change. |

## Concurrency decision

The identified worktree costs repeat work; additional goroutines would multiply Git processes and SQLite readers under concurrent sessions. Batch within one request and skip redundant Git commands first. Existing asynchronous create/exit operations own cancellation, and the PR service bounds concurrent network refreshes at four. No new goroutine is needed for the retained fixes.

## Baseline evidence collected

- `BenchmarkCollectCleanWorktreeStatus`: real temporary Git checkout with 256 tracked clean files, `-benchtime=200ms -count=5`, `GOMAXPROCS=2`. Baseline 36.011–52.578 ms/op (median 45.699), 678,184–688,958 B/op, 6,277–6,282 allocations/op. Raw: `.cache/performance/worktree-workspace/status-before.log`.
- `BenchmarkWorktreeAggregateInspection`: 64 worktrees across two real temporary SQLite workspace registrations and two temporary Git repositories. Fresh cached status and PR observations, warmed existing repository identity cache, `-benchtime=200ms -count=5`, `GOMAXPROCS=2`. Baseline 5.170–6.053 ms/op (median 5.620), median 856,387 B/op and 16,404 allocations/op; exactly 64 workspace resolutions, 64 PR reads and 64 status reads per operation. Raw: `.cache/performance/worktree-workspace/aggregate-before.log`.

## Retained implementation

1. Clean porcelain snapshots skip the two diff subprocesses. Dirty status, error-to-unknown persistence, branch/upstream and base divergence retain the existing path.
2. Aggregate authorization is resolved in the API owner before calling the service. The service filters both profile and allowed workspace before status/activity/PR inspection; an empty allowed set returns an empty listing without touching the store. See [API/task report](api-tasks.md) for HTTP/UDS boundary evidence.
3. Aggregate and workspace listings resolve each workspace once for PR hydration and read all exact `(workspace, repository, branch)` identities through the existing batch PR store method. The temporary resolution map belongs to one request; existing PR refresh/backoff and repository identity cache policies remain unchanged. PR #983's negative identity cache is reused, not replaced.
4. Catalog frames accumulate only authorized workspace IDs, then reread the workspace prefixes and aggregate prefix once per quiet window. The existing detailed-stream timer implementation is shared: 150 ms quiet window, 1 s ceiling during continuous traffic. Error/disposal flush outstanding work, and daemon failure text stays immediate.
5. Detail/exit invalidation targets the exact workspace list plus the affected detail, status and exit plan. Sibling worktree queries no longer refetch from that wake signal.

The narrow runtime identity request used by `use-user-home-dir.ts` is implemented and validated by the [daemon/runtime owner](daemon-runtime.md), after delegation of that hook and its existing test cases.

Owned changed files:

- `internal/worktree/aggregate.go`, `inspection.go`, `status.go`, `status_details.go`, `status_test.go`.
- `internal/store/globaldb/global_db_worktree_test.go` (canonical cases, shared real fixture and benchmark; no production store/SQL edit).
- `web/src/systems/workspace/lib/worktree-reread.ts`.
- `web/src/systems/workspace/hooks/use-worktree-catalog-stream.ts`, `use-worktree-stream.ts`, `use-worktree-exit.ts`.
- `web/src/systems/workspace/hooks/__tests__/use-worktree-catalog-stream.test.tsx`, `use-worktree-stream.test.tsx`.

## Measured results

| Fixture and observable | Before | After | Result |
| --- | ---: | ---: | --- |
| Clean real Git status, median of five runs | 45.699 ms/op | 16.169 ms/op | 64.6% less time |
| Clean status allocation, median | 679,556 B/op; 6,279 allocations/op | 226,674 B/op; 2,099 allocations/op | 66.6% less allocated memory and allocations |
| Catalog burst, 100 frames, three live query observers | 400 requests: list 100, aggregate 100, detail 200 | 3 requests: one each | 99.25% fewer requests; final generation remains 100 |
| Detail wake, four owned observers plus sibling | 8 requests: 1/2/2/2 plus sibling 1 | 4 requests: 1/1/1/1 plus sibling 0 | 50% fewer requests; owned generation advances, sibling remains unchanged |
| 64-row SQLite aggregate, median of five runs | 5.620 ms/op | 2.682 ms/op | 52.3% less time |
| Aggregate allocation, median | 856,387 B/op; 16,404 allocations/op | 520,530 B/op; 7,641 allocations/op | 39.2% less allocated memory; 53.4% fewer allocations |
| Aggregate workspace resolutions / PR reads / status reads | 64 / 64 / 64 | 2 / 1 / 64 | Shared workspace resolution and one PR batch; status semantics unchanged |

The frontend before/after uses the same observer fixture and pinned Bun 1.4.0. The baseline production source is the frozen `de5ab2f4d` tree; only the measurement tests and extracted timer constants/helper were copied into that diagnostic tree. Raw evidence: `streams-before-pinned.log`, `stream-after.log` under `.cache/performance/worktree-workspace/`. Earlier host-Bun diagnostics are retained but are not used for the paired claim. These are actual query-function invocation counts, not browser interaction-latency claims.

The SQLite fixture uses real local persistence and the existing repository identity cache. Its workspace resolver reads registrations, without the daemon's full runtime-config resolution or active-session guard. It measures this inspection portion, not total endpoint latency. Per-row status and active-session reads remain: no speculative transaction/cache lifetime or concurrency is introduced to remove them.

## Focused verification

All commands ran through `python3 .cache/performance/run-bounded.py` from the isolated repository root. The wrapper serializes task validation and sets `GOMAXPROCS=2`, `GOFLAGS=-p=2`, `TURBO_CONCURRENCY=1`; frontend delivery-runtime checks use Bun 1.4.0.

- Passed: `go test ./internal/worktree -run '^TestServiceStatus$' -bench '^BenchmarkCollectCleanWorktreeStatus$' -benchtime=200ms -count=5` (`status-after.log`).
- Passed: `env CGO_ENABLED=1 go test -race ./internal/worktree -run '^(TestServiceStatus|TestPullRequestService|TestServiceDiscovery)$' -count=1 -parallel=2` in 2.131 s (`worktree-race.log`).
- Passed: `bunx turbo run test --filter=./web --only -- src/systems/workspace/hooks/__tests__/use-worktree-catalog-stream.test.tsx src/systems/workspace/hooks/__tests__/use-worktree-stream.test.tsx src/systems/workspace/hooks/__tests__/use-worktree-exit.test.tsx --maxWorkers=1`: 28 tests in three canonical files (`stream-after.log`). Includes latest-generation reconciliation, continuous-frame ceiling, immediate failure text, pending flush on error/unmount, authorized-scope replacement, malformed/foreign frames and sibling isolation.
- Passed: frozen baseline observer replay with the same pinned runtime, root Turbo `--cwd=.cache/performance/baseline --filter=./web --only`, test selector `Should reconcile (a burst|each live)`: two measurement cases (`streams-before-pinned.log`).
- Passed: `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_(hydrate_ordered|exclude_unauthorized|resolve_the_current)' -bench '^BenchmarkWorktreeAggregateInspection$' -benchtime=200ms -count=5` in 6.126 s (`aggregate-after.log`). Three real SQLite/Git cases ran five times: exact identity/order and shared PR reads for aggregate/detail listings; authorization before hydration; new root/status truth and canceled callers. The fixture creates a second worktree with its profile owner at insertion, preserving the immutable-owner trigger.
- Passed: `env CGO_ENABLED=1 go test -race -gcflags=modernc.org/...=-d=checkptr=0 ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_(hydrate_ordered|exclude_unauthorized|resolve_the_current)' -count=1 -parallel=2` in 9.430 s (`aggregate-race.log`). The modernc-only checkptr flag is the repository's normal Go-lane setting from `magefiles/gotest_lane.go`; race remains enabled for the entire suite.
- Formatting: focused `gofmt` and `oxfmt` passed. The test-conventions heuristic passes `status_test.go`. On `global_db_worktree_test.go` it reports only the untouched `TestSessionBranchSnapshots` inline-case shape; running the same checker against the extracted `de5ab2f4d` file reproduces that finding (`test-conventions.log`, `test-conventions-baseline.log`). No new case fails the naming/parallelism convention.

Discarded validation attempts are distinguished from accepted proof: shared SQL generation temporarily lacked `ListTaskEventsForTasks`; the initial profile-scope fixture attempted an owner mutation correctly forbidden by `profile_owner_immutable`. The fixture was corrected to insert a row with its intended owner, without changing production constraints or weakening the scoped assertions. The final accepted commands above ran after regeneration and fixture repair.

## Impact and integration handoff

No persisted schema, public DTO, CLI verb, native-tool ID, hook/config contract or official `skills/compozy/` instruction changes. Workspace and profile authorization occurs before the expensive hydration; identity remains workspace/repository/branch-qualified and request-local sharing cannot outlive a root change. No additional goroutine, durable cache, pool, timer policy or unbounded event buffer is introduced.

Affected scenario owners: `ET-worktree-hook-event-contract`, `RT-worktree-web-nested-navigation`, `RT-worktree-web-exit-progress`, `RT-worktree-api-surface-parity`. The root owns scenario documentation and real integrated replay. Suggested existing real-Git integration cases under `TestWorktreeLifecycleIntegration`: `Should record real status corruption and recover truthful reads`, `Should report real upstream divergence exactly`, `Should refuse dirty and unpushed real loss then honor explicit evidence`, and `Should re-read real Git evidence after an external commit during removal`.

This worker did not launch a daemon/browser, run full suites/gates, or commit. Integrated lint/typecheck/codegen, existing integration journeys and current-head PR CI remain with the root owner; focused receipts do not substitute for those gates.
