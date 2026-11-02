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
| 64-row SQLite aggregate, median of five runs with every row checked | 6.292 ms/op | 2.628 ms/op | 58.2% less time |
| Aggregate allocation, median with every row checked | 856,428 B/op; 16,404 allocations/op | 520,816 B/op; 7,642 allocations/op | 39.2% less allocated memory; 53.4% fewer allocations |
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

## Build and development tooling coverage (2026-11-02)

This follow-up covers seven additional Go package directories outside `internal/`. These are development/build commands or test harnesses, not daemon request handlers or renderer interaction paths. The read-only inventory contains 52 Go files, including 44 under `magefiles`; exact paths are retained in `.cache/performance/worktree-workspace/build-tool-go-paths.txt`. The production Mage I/O/concurrency scan is retained in `.cache/performance/worktree-workspace/magefiles-io-scan.txt`.

No tooling command, package test, benchmark, development supervisor, publisher or gate was executed for this follow-up. The conclusions below are source inspection, not runtime or performance passes. No source change or measured tooling improvement is claimed. Existing tests identify the owning verification surface; their presence does not count as execution evidence.

### `magefiles`

Inspected execution owners: `magefiles/command.go`, `magefiles/verify.go`, `magefiles/verifylock.go`, `magefiles/test.go`, `magefiles/gotest_lane.go`, `magefiles/gotest_integration.go`, `magefiles/gotest_discovery.go`, `magefiles/gotest_census.go`, `magefiles/build.go`, `magefiles/codegen.go`, `magefiles/codegen_guard.go`, `magefiles/deps_lint.go`, `magefiles/deps_fmt_lane.go`, `magefiles/deps_lint_modules.go`, `magefiles/web.go`, `magefiles/e2e.go`, `magefiles/web_assets.go`, `magefiles/web_assets_publish.go` and the dependency-closure execution in `magefiles/boundaries.go`. Existing ownership was cross-checked in `magefiles/verifylock_test.go`, `magefiles/gotest_lane_test.go`, `magefiles/gotest_lane_env_test.go` and `magefiles/deps_lint_test.go`.

The earlier audit read `magefiles/gotest_lane.go` only to establish the repository's normal modernc race/checkptr policy. This entry completes explicit package coverage; the earlier worktree race receipt verifies use of that policy, not the Mage package itself.

A later CI investigation changed only diagnostic retention in `magefiles/test.go` and
the runtime uploader. Its before/after artifact probe and scoped Mage lint are recorded
in [daemon/runtime](daemon-runtime.md#ci-follow-up-hosted-mcp-prompt-timeout-and-retained-diagnostics).
That subsequent verification fix is separate from this static performance inspection.

`Verify` executes named steps sequentially under a machine-wide capacity lock. Lock polling owns and stops its ticker, reports the current holder, and releases the file lock on completion. Acquisition intentionally has no deadline; lock errors warn and continue because this is a capacity guard. Command wrappers use synchronous `exec.CommandContext`, but many top-level commands pass `context.Background`, so this is not a claim that every tool invocation has a deadline. Go test invocations add their own timeouts, cap package/per-package parallelism, split oversized suites and use deterministic weighted sharding. Lint caps concurrency by policy, isolates its versioned cache by repository root, and scopes local formatting to changed files. Codegen remains ordered around shared outputs; its checked marker avoids a redundant check later in the same verified pipeline.

Package discovery uses `go list` plus AST inspection instead of executing discovered tests. Asset freshness uses sorted content digests and immutable artifact identity rather than a TTL. Lane preparation reuses validated binaries, records cleanup and unwinds partial failures. Release/module-proxy polling has a fixed attempt count and a context-aware timer; its manual publishing entry point is not an interaction path. Directory hashing, dependency scans, compilation and the intentional double-build determinism check can cost developer time, but no measured redundant workload supports a change here. Adding subprocess concurrency would compete with existing bounded lanes and shared generation outputs. Disposition: preserve the current execution/capacity policy; static audit only.

### `scripts` (Go files at directory root)

Inspected `scripts/gate_integration_test.go` and `scripts/product_language_integration_test.go`. Both are integration-tagged test files; this directory has no production Go command. The gate suite owns evidence freshness, changed/generated input fingerprints, cache reuse, missing-log reruns, module scoping, fail-closed merge-base behavior and capacity-slot queuing. It seeds a Git fixture once per top-level suite and copies it into independent temporary directories for parallel cases. Subprocesses use test contexts and isolated capacity directories. The capacity-wait case uses a finite helper goroutine to release its fixture slot and consumes its result. Product-language cases use small temporary inputs and context-bound checker subprocesses.

The main cost is intentional shell/Git integration work in the test process. Independent fixture copies preserve concurrent case isolation; shared mutable fixtures or extra process fan-out would weaken it. No runtime cache or product lifecycle is owned here. Disposition: no supported optimization; the canonical integration suites were inspected but not run in this follow-up.

### `scripts/air-state-dir`

Inspected the complete `scripts/air-state-dir/main.go`. This one-shot developer command resolves the home directory once, appends `.dev/air`, prints the path and exits. Errors propagate to stderr and a failing exit code. There is no directory scan, daemon connection, polling, retained cache or goroutine. Disposition: constant-size local path work; no supported optimization. Static inspection only, with no command execution.

### `scripts/air-state-lock`

Inspected `scripts/air-state-lock/main.go` and process-lifetime cases/helpers in `scripts/air-state-lock/main_test.go`. The Unix-only developer wrapper validates the lock target, opens it with restrictive permissions, acquires an exclusive file lock and synchronously runs the supplied command. The descriptor is close-on-exec, so descendants cannot retain the lock after the wrapper exits; deferred close joins its error with the command result. The blocking lock acquisition does not take cancellation or a deadline, and `main` uses a background context. That wait deliberately serializes ownership of the development daemon state.

The existing tests own command-lifetime lock release and surviving-child descriptor isolation. Their subprocess cleanup uses a process group, termination, a bounded wait and a kill fallback; this source review does not establish a new runtime pass. Extra concurrency would violate the wrapper's ownership purpose. Disposition: no measured improvement and no source change; development-only static audit.

### `scripts/demo-seed`

Inspected the complete `scripts/demo-seed/main.go`, separately from the earlier `internal/demoseed` audit. The wrapper owns flag/home resolution, a signal-cancelable context and an exclusive daemon-state lock. It refuses to seed while a daemon owns the state, releases the lock with joined errors, calls `internal/demoseed.Seed` and emits either JSON or a bounded result summary. The implementation has no independent writer workers or persistent cache. Keeping dependent fixture writes under one ownership lock preserves consistency; there is no evidence that parallel writes would improve product responsiveness.

Disposition: explicit offline fixture setup, not a production navigation path. The existing `internal/demoseed` source conclusion is reused only for the delegated seeding work. This wrapper was not run and no user home or fixture database was modified.

### `scripts/dev-http-endpoint`

Inspected the complete `scripts/dev-http-endpoint/main.go`. The command resolves home paths and loads configuration once per invocation, maps wildcard bind addresses to loopback addresses usable by the caller, and prints host/port. It makes no HTTP request and starts no daemon. Configuration stays fresh through invocation-time loading; there is no retained cache, polling or goroutine to optimize. Disposition: small development preflight command, outside request/renderer paths. Static inspection only, with no config or runtime mutation.

### `scripts/devreadiness`

Inspected `scripts/devreadiness/dev_readiness_integration_test.go`, the directory's only Go file. It is a Unix-only test package, despite the integration filename; it has no production Go command. `TestDevDaemonReadiness`, `TestDevSupervisorReadiness` and `TestRunAir` own build/readiness identity and developer process coordination. Fixtures use temporary directories, real build/Git operations where appropriate and executable shims at external process boundaries. Supervisor tests give the command a context, own its asynchronous wait result and stop its process group with a termination/kill fallback. Readiness marker/run identity prevents stale readiness from starting the next process. These tests explicitly do not establish browser or daemon HTTP-health behavior.

One unmeasured harness lifecycle limitation was reported to the root owner: `readFIFOEvent` starts a goroutine whose `os.Open` or `Read` can remain blocked when the matching writer never arrives. `awaitFIFOEvent` stops its own timer and fails on timeout, but does not cancel or join that producer. The buffered result channel prevents a later send from blocking; it does not bound the FIFO open/read. This is a static failure-path observation, not a reproduced leak, measured regression or product impact claim. No test/tooling change was made under the read-only assignment. Disposition: process cleanup is explicit for launched commands; the FIFO helper's failure lifetime remains an identified limitation. No package tests or readiness harness were executed in this follow-up.

## CI exit-refresh fixture diagnosis (2026-11-02)

Before changing the existing fixture: the invariant is that a completed exit action persists fresh local status and requests exactly one forge refresh with the correct repository/branch identity, then retains the returned forge facts. The owning layer is worktree exit completion, and the canonical suite is `TestExitActions` in `internal/worktree/exit_actions_test.go`.

CI for `14730a705` failed `Should refresh local and forge status once after a completed action`; the exact subtest reproduced locally in 0.467 s. Its positional Git stub still supplied two empty responses for the now-omitted clean `diff` commands. `remote get-url origin` consumed the first empty response, so repository identity was unavailable and the forge refresh correctly could not be scheduled. The failure is in command-to-fixture response routing, not evidence that real Git returns an empty origin after the clean-status optimization. The repair uses the existing command-aware Git stub seam, retains all forge/cache assertions and verifies the persisted local status. No redundant production Git subprocess is restored to satisfy a positional stub.

Only `internal/worktree/exit_actions_test.go` changed for this repair; production sources remain unchanged. Final receipts, all through the bounded runner:

- Exact repaired case: `go test ./internal/worktree -run '^TestExitActions$/Should_refresh_local_and_forge_status_once_after_a_completed_action$' -count=1 -parallel=2` passed in 0.397 s. The unchanged forge assertion still requires one call, the normalized remote URL and the requested branch; the cached forge facts still must exist. The additional read-back requires the persisted local branch, zero dirty files, a refresh timestamp and no read failure.
- Negative control: the same case with `-overlay .cache/performance/worktree-workspace/exit-refresh-negative-overlay.json` failed in 0.369 s at the forge assertion with zero refresh calls. The overlay omits only `requestPullRequestRefresh` from `refreshAfterExit`; the checked-out production file was never edited. This confirms the repaired fixture still detects an actual missing forge refresh.
- Full package, including all exit cases: `env CGO_ENABLED=1 go test -race ./internal/worktree -count=1 -parallel=2` passed in 2.935 s.
- Focused `gofmt` completed, and the repository test-shape checker passed `internal/worktree/exit_actions_test.go`. No test inputs changed after these accepted runs.

The captured failure and raw follow-up receipts are `.cache/performance/worktree-workspace/exit-refresh-before.log`, `exit-refresh-after.log`, `exit-refresh-negative.log`, `exit-refresh-package-race.log` and `exit-refresh-conventions.log`. These establish the fixture diagnosis and package behavior; integrated browser evidence and current-head CI remain root-owned.

## PR #1141 review follow-up at `1665fb0c1`

Named invariants before changing coverage:

- A transient workspace-resolution failure for one row must not suppress cached PR facts for later rows in the same listing when resolution succeeds. A successful resolution remains shared for the remaining rows; repository identity still follows the existing `RepoKey` cache policy. Owner: worktree inspection hydration over the real SQLite repository. Canonical suite: `TestGlobalDBWorktreeStore` in `internal/store/globaldb/global_db_worktree_test.go`.
- Every timed aggregate benchmark operation must return all 64 rows with complete status and PR facts. Owner: `BenchmarkWorktreeAggregateInspection` in that same canonical file. The comparison must use the same guard, fixture, runtime and execution budget on both implementations.

Both findings are valid and fixed locally:

| Review thread | Evidence and disposition |
| --- | --- |
| `PRRT_kwDORy7nkc6yiTjZ` | Before the fix, the request-local map retained an empty entry after a resolver error. The new canonical case failed on the second row's missing PR facts. Production now skips that failed resolution without adding a map entry, allowing the ordinary next row to resolve again. Its success is shared with the third row. The existing `RepoKey` policy, including negative identity observations, remains unchanged. Fixed and verified locally; commit/current-head CI and GitHub thread resolution remain root-owned. |
| `PRRT_kwDORy7nkc6yiTjQ` | The benchmark previously checked only row zero. Every timed operation now checks all 64 rows, including status identity, nonempty status/PR facts and their fixture values. A test-only overlay that removes only the final row's PR facts fails at row 63. Both benchmark implementations use the identical strengthened guard. Fixed and verified locally; commit/current-head CI and GitHub thread resolution remain root-owned. |

The regression case uses three authorized rows, real SQLite persistence, real Git repository identity and a failure injected only at the workspace resolver I/O boundary. It retains local status for all rows, leaves the unresolved first branch without forge facts, hydrates both later branches, and observes two resolver calls, one PR batch and three status reads. No retry loop, background work, new timeout, TTL or cross-request state is added; only later rows already present in the same listing can attempt resolution again.

Focused receipts, all through `python3 .cache/performance/run-bounded.py`:

- Fail-before: `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_resolve_later_listing_rows_after_a_transient_workspace_failure$' -count=1 -parallel=2` failed on row 1 in 1.026 s (`resolve-transient-before.log`).
- After: `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_(hydrate_ordered|exclude_unauthorized|resolve_the_current|resolve_later_listing)' -count=1 -parallel=2` passed all four owning cases in 1.549 s (`resolve-transient-after.log`). The same selection with `env CGO_ENABLED=1`, `-race` and the repository's modernc-only `-gcflags=modernc.org/...=-d=checkptr=0` passed in 5.867 s (`resolve-transient-race.log`).
- Existing status/PR policy: `env CGO_ENABLED=1 go test -race ./internal/worktree -run '^(TestServiceStatus|TestPullRequestService)$' -count=1 -parallel=2` passed in 2.360 s (`resolve-transient-worktree-race.log`).
- Benchmark negative control: `go test -overlay .cache/performance/worktree-workspace/aggregate-last-row-negative-overlay.json ./internal/store/globaldb -run '^$' -bench '^BenchmarkWorktreeAggregateInspection$' -benchtime=1x -count=1` failed with `incomplete listing row 63` in 1.038 s (`aggregate-last-row-negative.log`). The overlay leaves row zero intact and never changes the checked-out production file.
- Focused `gofmt` completed. The test-shape checker reports only the unchanged `TestSessionBranchSnapshots` inline-case shape, matching the already archived `de5ab2f4d` result; no new case is flagged (`resolve-transient-conventions.log`, `test-conventions-baseline.log`).

The updated benchmark pair uses `-run '^$' -bench '^BenchmarkWorktreeAggregateInspection$' -benchtime=200ms -count=5`, Go 1.27.3 on darwin/arm64 and the same bounded runner. The baseline command runs with `go -C .cache/performance/baseline test -overlay <absolute aggregate-full-guard-baseline-overlay.json>` against the frozen `de5ab2f4d` production tree. Only the test harness is overlaid, adapting the former two-argument `ListAggregate` call; the guard bytes match the current benchmark exactly. The baseline aggregate/inspection/status-details source bytes were checked against `de5ab2f4d`.

The new median is 6.292139 → 2.628459 ms/op (58.23% less time), 856,428 → 520,816 B/op (39.19% less allocation) and 16,404 → 7,642 allocations/op (53.41% fewer allocations). Workspace/PR/status reads remain 64/64/64 → 2/1/64. These values replace the earlier aggregate timing/allocation pair in the results table; earlier receipts remain historical evidence from the first guard. Raw pair: `aggregate-full-guard-before.log`, `aggregate-full-guard-after.log`; computed medians, equal-guard verification, runtime and final source hashes: `review-worktree-summary.json`, all under `.cache/performance/worktree-workspace/`.

One discarded baseline attempt named the symlink rather than the canonical source path in its Go overlay, so no benchmark was discovered. Its zero-case output is retained separately as `aggregate-full-guard-before-no-benchmark.log` and is excluded from the comparison. The accepted canonical-path run contains all five benchmark samples. No production/test input changed after the accepted checks; no commit or full gate was run by this worker.

## PR #1141 transient repository-identity follow-up at `e8046c842`

Before changing coverage: the invariant is that an uncached repository-identity failure on one row cannot suppress PR facts for later rows that can successfully resolve the same repository. Successful rows still share their repository identity and one PR batch. Owner: worktree inspection hydration over real SQLite and Git. Canonical suite: the existing transient-listing cases in `TestGlobalDBWorktreeStore`, `internal/store/globaldb/global_db_worktree_test.go`. The existing `TestPullRequestService` owns stable negative identity caching, expiry and interruption semantics; those assertions remain unchanged.

Thread `PRRT_kwDORy7nkc6yist0` identifies a different error boundary from the earlier workspace resolver fix. `RepoKey` does cache Git exit errors and unsupported/missing origin observations for one minute. It deliberately does not cache non-exit runner/start errors or canceled/deadline reads. `RealGitRunner` returns a wrapped start error when a checkout directory disappears before `cmd.Start`, so later rows can recover within the same listing if the directory returns. The canonical reproduction temporarily moves only the fixture's checkout root away before the first identity lookup and restores it at the next workspace resolution, using the real Git runner throughout. Cleanup also restores that temporary root when the fail-before assertion stops the case.

The finding is valid. Before the production edit, the new repository-failure variant failed on row 1's missing PR facts in 0.907 s. It shares the existing transient workspace case's observable assertions: all three rows keep their cached status, the failed first branch has no PR facts, both later branches recover, and the successful identity is shared thereafter. The recovery proves this real Git failure was not retained by `RepoKey`; the listing's empty memo entry was the suppression point.

`hydrateDisplayPullRequests` now skips a failed `RepoKey` result before inserting into the request-local map. Only successful keys are shared. `pull_request_identity.go` is unchanged: stable unsupported/missing origins still reuse its one-minute negative cache, and cancellation/runner failures retain their existing treatment. There is no retry loop, new concurrency or work outside the rows already in the current listing. Normal successful workspace/PR batching is preserved. The existing transient case became a two-variant table in the same canonical file; no new test file or production abstraction was introduced.

All receipts below used `python3 .cache/performance/run-bounded.py`, with `GOMAXPROCS=2` and `GOFLAGS=-p=2`:

| Check | Command / selection | Result and receipt |
| --- | --- | --- |
| Fail-before, unchanged `e8046c842` production | `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_resolve_later_listing_rows_after_a_transient_repository_failure$' -count=1 -parallel=2` | Expected missing-PR failure on row 1, 0.907 s; `repokey-transient-before.log` |
| Canonical behavior after fix | `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_(hydrate_ordered\|exclude_unauthorized\|resolve_the_current\|resolve_later_listing)' -count=1 -parallel=2` | All five cases passed, 1.101 s; `repokey-transient-after.log` |
| Race plus 64-row completeness/count check | Same canonical selection with `env CGO_ENABLED=1`, `-race -gcflags=modernc.org/...=-d=checkptr=0 -bench '^BenchmarkWorktreeAggregateInspection$' -benchtime=1x` | Passed, 6.538 s; exactly 2 workspace resolutions, 1 PR read and 64 status reads; `repokey-transient-race-counts.log` |
| Existing repository identity and status policies | `env CGO_ENABLED=1 go test -race ./internal/worktree -run '^(TestServiceStatus\|TestPullRequestService)$' -count=1 -parallel=2` | Passed, 1.989 s, including stable negative-cache expiry and interruption recovery; `repokey-policy-race.log` |

The benchmark body and fixture are byte-identical to `e8046c842`, and every returned row still passes its full status/PR guard. This single race-instrumented benchmark iteration verifies completeness and read counts only; it is not a new timing or allocation comparison. The previously paired timing measurements remain separate evidence. File hashes and the unchanged-fixture/policy comparison are retained in `repokey-transient-freeze.json` under `.cache/performance/worktree-workspace/`.

Focused `gofmt` completed. The test-shape checker reports only the unchanged `TestSessionBranchSnapshots` inline-case shape, already reproduced against `de5ab2f4d`; no new case is flagged (`repokey-transient-conventions.log`). Production and test inputs are frozen after the accepted checks. The thread disposition is valid/fixed locally; current-head CI, commit and GitHub thread resolution remain with the root owner. No provider credential-fence source, generated file, schema or public contract was changed.

## PR #1141 PR batch failure-isolation triage at `862bf0fd0`

Before changing diagnostic coverage: the invariant is that one cached PR row that cannot be decoded must not suppress independently readable PR facts for other authorized rows; local status and listing order remain intact. Owner: worktree listing hydration over the real SQLite PR repository. Canonical suite: `TestGlobalDBWorktreeStore` in `internal/store/globaldb/global_db_worktree_test.go`, reusing its real SQLite/Git inspection fixture. Initial reproduction is test-only through Go overlays; production remains untouched during triage.

Thread `PRRT_kwDORy7nkc6yjhQe` points to a batch error boundary. `WorktreeRepo.ReadPullRequests` returns `nil, err` on a query or row-decoding error, including after earlier rows have decoded successfully. `PullRequestService.Read` suppresses that error as a nil result and does not schedule refreshes on the failure path. The SQLite schema accepts any valid JSON in `checks_failed_names`, but the row mapper requires `[]string`; a stored `{}` therefore provides a real, persistent per-row decoding failure without violating the schema or changing production code. Retrying the same batch would encounter that row again. The diagnostic compares one malformed row and five valid cached rows against the current batched implementation and the frozen `de5ab2f4d` per-row implementation.

The identical behavioral case failed on unchanged `862bf0fd0` production (0 of 5 readable facts, one PR read; 0.917 s) and passed against the frozen `de5ab2f4d` per-row implementation (all 5 readable facts, six PR reads; 0.859 s). Both real batch probes returned the same decoding error and zero partial entries. These test-only overlays changed no production source. Receipts: `pr-batch-failure-current.log` and `pr-batch-failure-baseline.log` under `.cache/performance/worktree-workspace/`. The root approved a per-key fallback after this causal comparison.

Additional invariants before materializing coverage: successful batches keep one PR read; failed multi-key batches attempt each key at most once; single-key failures are not retried; cancellation before or during fallback prevents further PR I/O. These belong to the same hydration layer and canonical SQLite fixture, with cancellation injected at its existing store I/O boundary.

The fix remains entirely in `hydrateDisplayPullRequests`: a nil result from a multi-key batch enables one pass of individual reads. Results are recorded by exact key, including failed attempts, so duplicate keys are not retried. Context cancellation is checked before starting and before each individual read. A successful batch and a failed single-key lookup retain their original one-read path. `PullRequestService.Read`, its refresh scheduler, repository identity caches, store implementation, schema and public response shapes are unchanged. The malformed observation remains unavailable; independently readable observations recover without retrying the same broken batch or hiding its error through fabricated data.

The owning canonical suite now includes the persistent malformed-row case, cancellation at the initial batch, cancellation during the first individual read and a failing single-row listing. Its real SQLite mapper still receives the malformed JSON; only the cancellation signal is injected at the store boundary. The first case recovers all five good observations with seven reads (one failed batch plus six individual keys). Cancellation cases stop at one and two reads respectively; the single-row case remains one failed read. Listing order and all local status identities stay intact. Existing successful, scoped and transient-resolution cases remain in the same run.

All accepted checks used `python3 .cache/performance/run-bounded.py`:

- `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_(retain_valid_PR|stop_PR_fallback|not_repeat_a_failed_PR|hydrate_ordered|exclude_unauthorized|resolve_the_current|resolve_later_listing)' -count=1 -parallel=2` passed nine canonical cases in 1.323 s (`pr-batch-failure-after.log`).
- The same selection with `env CGO_ENABLED=1`, `-race -gcflags=modernc.org/...=-d=checkptr=0 -bench '^BenchmarkWorktreeAggregateInspection$' -benchtime=1x` passed in 9.522 s (`pr-batch-failure-race-counts.log`). Every one of the 64 normal rows retained complete status/PR facts, with exactly 2 workspace resolutions, 1 PR read and 64 status reads.
- `env CGO_ENABLED=1 go test -race ./internal/worktree -run '^(TestServiceStatus|TestPullRequestService)$' -count=1 -parallel=2` passed in 2.184 s (`pr-batch-policy-race.log`).
- Focused `gofmt` completed. The test-shape checker again reports only the unchanged `TestSessionBranchSnapshots` shape, previously demonstrated against `de5ab2f4d`; new cases are not flagged (`pr-batch-failure-conventions.log`).

The benchmark iteration checks completeness and read counts under race; it supplies no new timing or allocation comparison. Its body/guard are unchanged, while the test fixture now has an optional cancellation callback that is nil for normal benchmark reads. Earlier timing pairs remain historical measurements with their own recorded fixtures. Raw receipts are under `.cache/performance/worktree-workspace/`; the diagnostic overlays retain the causal current/baseline comparison without changing either production tree during reproduction.

Thread disposition: valid and fixed locally. `RT-worktree-api-surface-parity` has an append-only note for this cached-observation isolation behavior. Source and tests are frozen after the accepted checks. Integrated lint, transport/runtime replay, current-head CI, commits and GitHub thread resolution remain with the root owner; this worker ran no full gate and made no commit or push.

Formatting follow-up: the root's scoped lint receipt (`.cache/performance/recovery-go-lint.log`) found formatter drift in the new SQLite `ExecContext` call. The bounded runner applied `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt internal/worktree/status_details.go internal/store/globaldb/global_db_worktree_test.go` successfully (`pr-batch-failure-format.log`). Only the call's line breaks changed; production source, assertions and benchmark body are unchanged. No behavioral check was repeated for this formatting-only edit. The freeze manifest was refreshed; the root owns the integrated lint rerun.

## PR #1141 typed corruption proof and fallback cost at `bfb1cc476`

Before changing coverage for thread `PRRT_kwDORy7nkc6ykJMJ`: the invariant is that the malformed-row fixture reaches a wrapped `*json.UnmarshalTypeError`, rather than merely returning an unrelated non-nil error. Owner: the real SQLite PR decoder used by worktree listing hydration. Canonical suite: the existing malformed-row table in `TestGlobalDBWorktreeStore`, `internal/store/globaldb/global_db_worktree_test.go`. The added `errors.AsType[*json.UnmarshalTypeError]` assertion follows the repository's Go 1.27 guidance and retains every existing assertion, including the non-nil check, listing order, local statuses, recovered PR facts and I/O counts. No production source or fixture data changes.

Read-only disposition for thread `PRRT_kwDORy7nkc6ykJMR`: retain the bounded fallback; a store/partial-decoding redesign is not supported by measured normal-path benefit. The paired normal 64-row receipts (`aggregate-full-guard-before.log`, `aggregate-full-guard-after.log`) show 64 → 1 PR reads, and `pr-batch-failure-race-counts.log` confirms the current recovery code retains one normal batch with all 64 complete rows. The persistent malformed-row receipt on the original per-row baseline (`pr-batch-failure-baseline.log`) shows 5 valid facts with 6 PR reads. The current canonical case asserts the same 5 valid facts with 7 reads: one failed batch plus one attempt for each of 6 distinct keys. Thus this observed recovery path costs one additional read over the original implementation; no corruption frequency or timing benefit is claimed.

The current canonical cancellation cases require exactly 1 read when canceled in the batch and 2 when canceled during the first individual attempt. The failed single-key case requires exactly 1 read. Duplicate keys are skipped after their first fallback attempt. Static review confirms `WorktreeRepo.ReadPullRequests` still discards a batch on decode error, `PullRequestService.Read` still returns nil on store errors, and `Refresh` still propagates them; no API, stale-observation refresh policy or store error semantics were changed. Replacing those contracts to remove fallback reads would widen this fix without evidence of a normal-path gain.

Focused evidence, all through `python3 .cache/performance/run-bounded.py`:

- Canonical formatter `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt internal/store/globaldb/global_db_worktree_test.go` completed before validation (`pr-batch-error-type-format.log`).
- `go test ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_retain_valid_PR_facts_when_another_cached_row_cannot_be_decoded$' -count=1 -parallel=2 -v` passed against the real SQLite/Git fixture in 0.906 s (`pr-batch-error-type-focused.log`).
- `env CGO_ENABLED=1 go test -race -gcflags=modernc.org/...=-d=checkptr=0 ./internal/store/globaldb -run '^TestGlobalDBWorktreeStore$/Should_(retain_valid_PR|stop_PR_fallback|not_repeat_a_failed_PR)' -count=1 -parallel=2 -v` passed all four cases in 4.417 s (`pr-batch-error-type-race.log`). These retain the 7/1/2/1 read-count assertions described above.
- A test-only Go overlay replaces the probe error with a non-wrapping error that retains the original decoder message. The exact malformed-row case fails at the new typed assertion in 0.870 s (`pr-batch-error-type-negative.log`). The same overlay with only the new assertion removed passes in 0.880 s (`pr-batch-error-type-prior-assertion.log`), proving the old non-nil assertion admitted this wrong error type. Neither overlay changes checked-out source or production.
- The test-shape checker still reports only the unchanged `TestSessionBranchSnapshots` inline-case finding, matching the archived original-baseline receipt (`pr-batch-error-type-conventions.log`, `test-conventions-baseline.log`).

Final disposition: `PRRT_kwDORy7nkc6ykJMJ` is addressed by the stronger typed proof; `PRRT_kwDORy7nkc6ykJMR` does not justify an additional production redesign. Production hydration, PR service and store bytes plus the benchmark body/guard remain identical to `bfb1cc476`; the normal-path read-count receipts are reused, not rerun or presented as new timing measurements. Final hashes are in `.cache/performance/worktree-workspace/pr-batch-error-type-freeze.json`. No new suite, production API, full gate, commit or push was added; integrated lint and current-head CI remain with the root owner.

## Main merge audit: `8c025ff8f` agent memory and workspace deltas

This section audits the incoming main changes during the root-owned merge, separately from the historical optimization measurements above. It adds source coverage and focused integration evidence; it does not claim a new latency or allocation improvement. No test or production repair was needed from static review. No new suite was introduced.

| Invariant | Owning layer and existing canonical suite |
| --- | --- |
| Profile/origin/agent isolation, digest conflicts, redaction, ordered archival, bounded startup/entries/history, safe filesystem access and serialized mutations | `internal/agentmemory`: `TestStore` and `TestMemoryBoundaries` in `store_test.go` |
| Origin identity cannot widen the existing persisted access ceiling | `internal/workspaceaccess`: `TestUnrestrictedGrant` in `default_policy_test.go` |
| Pointer-based session registration preserves missing-workspace history and seeded persisted sessions | `internal/workspace`: `TestResolverIntegrationListPreservesMissingWorkspaceAcrossReopen`; `internal/demoseed`: `TestSeed/Should persist one coherent scenario and replace it without duplication` |
| Worktree binding fences and complete ordered PR hydration survive the merged registration callsites, including typed malformed-JSON proof | `internal/store/globaldb`: existing focused cases in `TestGlobalDBWorktreeStore` |
| Agent identity fixtures remain compatible with workspace details, and keyboard entry waits for the existing input focus lifecycle | `web/src/systems/workspace`: `use-workspaces.test.tsx` and `workspace-command-select.test.tsx` |

`internal/agentmemory` received a complete source review: all five Go files (1,042 lines) and its README, with no omitted child package. The README and `skills/compozy/references/agent-memory.md` agree that authorization and stays-on eligibility belong to the shared service boundary; the store receives a resolved immutable profile ID, origin and agent. The API target resolver and session-search authorization path were inspected as boundary context, not claimed as fully audited packages here.

| Exact inspected file | Audit conclusion and performance/lifecycle boundary |
| --- | --- |
| `internal/agentmemory/store.go` | Keys reject separators, NUL and dot traversal; resolved home and directory checks combine with `fileutil` non-following regular-file/atomic-write helpers. One daemon-owned store serializes each exact key while allowing unrelated keys to proceed. Digest-checked writes, redaction and archive-before-main ordering remain intact. No goroutines are owned here. The lock map lasts for the store lifetime, and filesystem reads have no context or input-size bound; no speculative eviction or new I/O contract was added. |
| `internal/agentmemory/format.go` | Enforces 1,000-character tool entries and 200-line/24-KiB startup bounds. Stable date ordering archives the oldest dated entries and preserves undated notes; oversized undated content is bounded for startup without rewriting disk. Overflow reconstructs the remaining lines while evicting entries, so oversized externally edited files can cost more than ordinary bounded notes; this is a static scaling limit, not a measured regression. |
| `internal/agentmemory/history.go` | Retains 50 snapshots through the existing atomic file helper; rollback accepts only enumerated revision IDs and reuses redaction/budget enforcement. Retirement renames under the same key lock and syncs the parent directory. Separate store instances/processes are outside this in-process serialization contract. |
| `internal/agentmemory/search.go` | Reads current notes then archive, redacts before matching, ranks matched-term fractions and preserves source/line order for equal scores. Search scans the complete archive and accumulates all matches while holding the key lock; archive size and result count are not bounded by the startup limit. Replace validates the resulting entry and Remove uses the same serialized update path. No cache, parallel scan or search redesign was introduced without measurements. |
| `internal/agentmemory/store_test.go` | Read in full: the two canonical suites exercise real temporary files, reopen/rollback, 20 concurrent adds, origin isolation, retention, Unicode limits, redaction expansion, rejection without mutation, interrupted archival and symlink/path rejection. No assertions were changed. |
| `internal/agentmemory/README.md` | Read in full: lifecycle, crash ordering, bounds, authorization ownership and public-suite mapping match the inspected implementation. API/native/startup/browser paths listed here remain separate from the focused store evidence. |

| Package/system and exact delta | Integration disposition |
| --- | --- |
| `internal/demoseed/seed_sessions.go` | Adapts the seed registration literal to `RegisterSession(*store.SessionInfo)`. Metadata/transcript sequencing and session fields are unchanged; the receiving method copies the value before normalization. This is development/demo tooling, not a new runtime loading optimization. |
| `internal/workspace/resolver_integration_test.go` | Passes the existing fixture session by address. The owning reopen/missing-folder assertions remain unchanged; no resolver production or cache policy changed. |
| `internal/workspaceaccess/grant.go` | Adds `Origin.ID` as identity data. `Effective`, `NarrowGrant`, context propagation and `AuthoringCeiling` are unchanged, so the ID alone confers no access ceiling. |
| `web/src/systems/workspace/components/__tests__/workspace-command-select.test.tsx` | Adds `waitFor` on input focus before sending the navigation key; the panel focus and return-to-trigger assertions remain intact. No production navigation behavior changed. |
| `web/src/systems/workspace/hooks/__tests__/use-workspaces.test.tsx` | Supplies required agent label, kind and resolved appearance in the existing detail-response fixture. Fetch identity and abort-signal assertions remain intact. |
| `web/src/systems/workspace/mocks/fixtures.ts` | Supplies the same identity fields for the shared story/detail agents. No query keys, invalidations, stream batching or runtime state changed. |
| `internal/store/globaldb/global_db_worktree_test.go` | Automatic merge only adapts six registration callsites to pointers. The typed `*json.UnmarshalTypeError` assertion and every recovery, cancellation, single-key, ordering and binding assertion survive. `status_details.go`, `pull_requests.go` and `pull_request_identity.go` remain byte-identical to the pre-merge PR head. |

Accepted focused checks, serialized through `python3 .cache/performance/run-bounded.py`:

| Command after the bounded runner | Result / receipt under `.cache/performance/worktree-workspace/` |
| --- | --- |
| `env CGO_ENABLED=1 go test -race ./internal/agentmemory -run '^(TestStore\|TestMemoryBoundaries)$' -count=1 -parallel=2` | Both canonical filesystem suites passed, 3.709 s; `main-merge-agentmemory-race.log`. |
| `env CGO_ENABLED=1 go test -race -gcflags=modernc.org/...=-d=checkptr=0 ./internal/workspaceaccess ./internal/store/globaldb -run '^(TestUnrestrictedGrant\|TestGlobalDBWorktreeStore)$/Should_(authorize_and_audit_granted\|cap_every\|retain_valid_PR\|stop_PR_fallback\|not_repeat_a_failed_PR\|hydrate_ordered\|retain_missing_worktrees\|fence_new_session_bindings\|change_existing_session_bindings)' -count=1 -parallel=2` | Grant cases passed, 1.309 s; eight worktree cases passed, 5.788 s; `main-merge-worktree-grant-race.log`. |
| `env CGO_ENABLED=1 go test -race -tags=integration -gcflags=modernc.org/...=-d=checkptr=0 ./internal/workspace ./internal/demoseed -run '^(TestResolverIntegrationListPreservesMissingWorkspaceAcrossReopen\|TestSeed)$/Should_(retain_missing_registration\|persist_one_coherent_scenario)' -count=1 -parallel=2` | Real SQLite/filesystem workspace reopen passed, 2.966 s; demo scenario seed/replace/reopen passed, 15.728 s; `main-merge-workspace-seed-race.log`. |
| `bunx turbo run test --filter=./web --only -- src/systems/workspace/components/__tests__/workspace-command-select.test.tsx src/systems/workspace/hooks/__tests__/use-workspaces.test.tsx --maxWorkers=1` | 27 tests in two files passed; uncached Turbo task 6.379 s; `main-merge-workspace-web-tests.log`. Uses pinned Bun 1.4.0. |

Disposition: no concrete integration defect found in this owned slice, and no production/test edits were needed beyond the incoming merge. The only worker edit is this audit report. Source hashes and the equality checks against incoming main/pre-merge PR inputs are recorded in `main-merge-freeze.json`. These checks cover real filesystem, SQLite and renderer test boundaries, not an interactive browser, native transport or running-daemon replay. They also do not prove bounded latency for arbitrarily large externally edited archives, cross-process memory writes or cancellation of synchronous memory I/O. Those static limits remain explicit; no speculative cache, concurrency mechanism, retention policy or approved Stays-on contract was changed. Root owns integrated lint/codegen, current-head CI and merge/commit/publication.
