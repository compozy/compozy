# Session responsiveness investigation — 2026-10-26

Status: implementation and focused verification complete; final CI delivery pending. Branch `perf/session-responsiveness`, isolated checkout
`/Users/pedronauck/Dev/compozy/_worktrees/session-responsiveness`, baseline
`14136a511` (v0.4.2). The original checkout is unchanged.

The goal is to improve responsiveness under many concurrent sessions across database
queries, frontend loading, state propagation, and memory. This report records measured
changes and their verification limits; current-head delivery checks are tracked on PR #983.

## Measurement method

Local measurements use an Apple M4 Max. Go benchmarks use `GOMAXPROCS=2`, package
parallelism 1, `-benchtime=300ms -count=3 -benchmem`. Reported Go times are medians of
three runs. SQLite fixtures are real temporary databases using the shipped migrations.
Fixture setup and migration time are outside the benchmark timer. No operator database,
provider home, credentials, or running desktop process is modified.

The frontend probe uses the installed TanStack Query and Bun, 128 sessions with eight
cached reads each (1,024 entries), 64 invalidations per sample, three warmups, and ten
measured samples. It measures warm-cache CPU work, with inactive queries and no network.
It does not measure browser INP, frame rate, API/network latency, or production traffic.
The regression suite separately exercises mounted observers and verifies actual rereads.

## Measured changes

| Operation | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| Manager page, 1,024 persisted / 64 active sessions / 50 returned | 2.243 ms | 1.676 ms | 25.3% |
| Allocated bytes per manager page | 2,722,469 B | 767,867 B | 71.8% |
| Allocations per manager page | 8,246 | 3,556 | 56.9% |
| Detail family read, 64 active sessions | 0.276 ms | 0.252 ms | 8.6% |
| Allocated bytes per detail family read | 98,716 B | 20,371 B | 79.4% |
| Allocations per detail family read | 287 | 94 | 67.2% |
| Board SQL facets, 1,024 sessions / 16 repositories | 7.956 ms | 5.291 ms | 33.5% |
| Board SQL page including exact total, same population | 7.702 ms | 4.882 ms | 36.6% |
| 64 frontend live-surface wakes, 1,024 cached queries | 120.478 ms | 6.211 ms | 94.8% |
| 64 frontend usage wakes, same cache | 81.300 ms | 4.854 ms | 94.0% |
| 64 catalog reconciliation wakes, same cache | 96.827 ms | 12.240 ms | 87.4% |
| Global catalog HTTP page, 16 workspaces without a hosted origin | 170.488 ms | 3.611 ms | 97.9% |
| Global board HTTP page, same population | 172.680 ms | 5.698 ms | 96.7% |
| Global facets HTTP read, same population | 171.034 ms | 4.203 ms | 97.5% |
| Workspace catalog HTTP page, same population | 14.059 ms | 2.182 ms | 84.5% |
| Browser JS heap after 80 selections and settling, 64 active sessions | 970.64 MB | 77.49 MB | 92.0% |

The manager page measurements isolate the sort change, before the later SQL change.
Before/after ranges were 2.240–2.245 / 1.673–1.690 ms for that page, 7.941–7.986 /
5.202–5.298 ms for facets, and 7.691–7.755 / 4.802–5.009 ms for the board page.
Frontend ranges were 118.569–127.874 / 6.006–6.764 ms for live wakes and
80.257–82.113 / 4.550–5.164 ms for usage wakes. These differences exceed observed
run-to-run variation. Absolute timings are fixture-local, not production promises.

### Catalog sort allocations

The comparator previously constructed a complete session read model, including cloned
lineage and other nested data, every time two candidates were compared. A heap profile
attributed 74.4% of allocated bytes to candidate merging/sorting. Each candidate now
computes its existing ordering key once; sorting moves small references and keys before
materializing the ordered result. All sort precedence, tie breakers, active replacement,
page cuts, and cursor encoding retain their existing owner.

### Repository-scoped board SQL

Every candidate previously scanned the entire repository JSON object to find one key,
then walked the outer default-branch object before walking that project's branch list.
The query now extracts the repository by its quoted workspace key and starts `json_each`
at that workspace's branch array. The existing `(workspace_id, repo_key, branch)` index
continues to serve the PR join. There is no new index, schema, migration, or cache.

`EXPLAIN QUERY PLAN` removed the repository correlated scalar subquery and the
`default_workspace` virtual-table scan. The remaining default-branch scan uses virtual
table index 3 (a scoped root) instead of index 1 (the whole object). The owning suite
covers worktree lifecycle, detached heads, repository isolation, default branches,
parent/child scope, all badge/PR combinations, and pagination before the cut. Added
cases exercise workspace keys containing dots, brackets, quotes, and backslashes,
plus a missing repository key.

### Frontend cache propagation

A live wake previously called `invalidateQueries` five times, and a usage wake twice.
Each call walks the cache to invalidate and again to refetch. Exact-key matching also
recomputed serialized key hashes across unrelated entries. The existing session
invalidation owner now matches the entire wake in one predicate and invokes TanStack
once. Canonical key factories and string-scope matching remain authoritative.

Live wakes still update detail, history, input queue, all profile lenses for that
session, and its workspace catalog. Usage wakes still update only usage and usage turns.
Mutation wakes still include the session detail subtree. Inactive reads become stale;
active matching reads refetch once. Other sessions, other workspaces, aggregate catalog
scope, and transcript/Goal on live wakes remain unaffected. Usage stream batching keeps
its 250 ms debounce and flush-on-detach behavior, now reusing the canonical invalidator.

Catalog event reconciliation also avoids exact-key hashing over the entire cache and
looks up pending detail/by-ID reads in one traversal. Its trailing authoritative read,
post-stop transcript obligation, and profile teardown fence remain unchanged. The
existing catalog-stream suite owns these contracts. The 64-wake benchmark measured
96.219–98.356 ms before and 11.876–12.622 ms after. This is CPU work, not network time.

### Missing and local Git origins

The existing repository identity cache stored only successful hosted origins. A
workspace without `origin`, or with a local-path origin, spawned Git on every catalog
and detail read. A global catalog repeated this across every workspace. The same
per-root one-minute identity window now also stores Git exit failures and origins
that cannot identify a hosted repository. Cancellation, deadline and runner-start
errors remain immediately retryable. No provider/forge response freshness changes.

Real Git warm-read medians fell from 8.734 ms (missing origin) and 8.490 ms (local
origin) to about 40 ns, with zero allocations on the warm path. The already-cached
hosted-origin case stayed around 39–40 ns. These microbenchmarks exclude the first
Git command. The real HTTP table above uses the same daemon database, three warmups,
then ten measured requests per endpoint; response IDs, order and facets were compared
exactly. Baseline is the first optimization commit, `9e45483fb`, so those gains isolate
the negative identity cache. A newly configured origin can take up to one minute to
appear, matching the existing positive identity window. The owning real-Git suite
verifies expiration, recovery, root isolation and immediate retry after interruptions.

### Runtime stream ownership and retained memory

An initial production-browser walk exposed retained detached React trees: 80 session
selections grew the collected heap to 947 MB. A repeat without CPU profiling or
background mutations reproduced the growth. A sanitized heap snapshot traced a
strong retaining path from Chromium pending activities through
`ReadableStreamDefaultReader` → `PipeToEngine` → `ToolInvocationTracker._getTools` →
external-store adapter → React dispatch → detached session trees.

The pinned `@assistant-ui/core@0.3.22` eagerly opened a never-ending pipeline in the
tracker constructor. Abandoned React renders never committed an effect cleanup;
ordinary thread disposal also left the pipeline open. A small Bun dependency patch
now creates it on the first live tool entry and aborts it through the runtime's
existing permanent disposal signal. Disabling tool invocations also disposes it.
Reset and prompt cancellation remain reusable; late callbacks cannot update a disposed
owner. `patches/README.md` records the boundary, evidence and removal condition.

The regression in the existing chat-provider suite first failed with one pending
pipeline after an abandoned Suspense render. It now passes, including StrictMode
repeated mounts, processing streamed backend tool calls, and zero pending pipelines
after each unmount. The full 73-test provider suite also exercises prompt recovery,
drafts, cancellation, transcript handoff, attachments and session identity.

The controlled browser comparison rebuilt the same production tree with and without
only this dependency patch. Both runs used Chromium 1243, a 1,500 × 1,050 viewport,
one session window, 64 real active idle sessions, 1,076 retained catalog fixtures and
16 isolated workspaces. There were 16 first selections and four passes over the same
16 sessions (80 selections total), no CPU profiler and no concurrent build/test load.
Memory samples explicitly collected garbage; the final sample followed 2.5 seconds
of settling. No collection call or retention cap was added to the product.

| Browser checkpoint | Before heap | After heap | Before listeners | After listeners |
| --- | ---: | ---: | ---: | ---: |
| Initial | 102.09 MB | 57.53 MB | 2,876 | 1,413 |
| 16 selections | 114.77 MB | 68.41 MB | 2,889 | 1,424 |
| 48 selections | 566.64 MB | 71.96 MB | 16,369 | 1,427 |
| 80 selections | 969.62 MB | 75.63 MB | 28,002 | 1,428 |
| Settled | 970.64 MB | 77.49 MB | 28,000 | 1,426 |

Attached element count stayed exactly 2,998 on both sides. Total DOM nodes went from
32,121 to 153,911 before; after, they stayed near 24,773. Warm click-to-content medians
were 515.5 / 499.3 ms and p95 565.9 / 549.2 ms. That small timing difference is not
claimed as a significant latency improvement: these samples include Playwright action
stability and readiness assertions. Browser long-task totals were 5,188 / 4,283 ms
(76 / 63 tasks), also diagnostic rather than a production INP/SLO claim. Both runs
had zero page errors. The heap result concerns JavaScript retained memory, not process
RSS or provider memory.

A second independent patched run finished at 77.12 MB, 24,773 DOM nodes and 1,425
listeners after the same 80 selections. The 77.12–77.49 MB repeat range supports the
retention result; it does not establish a fixed memory ceiling for longer or larger
transcripts. These sessions were active and idle, not 64 simultaneous LLM generations.

### Detail family projection

The detail read filters the active session inventory by parent/direct-child identity
before constructing rich snapshots. Previously every detail relation read cloned and
sorted all active sessions, even when the requested session had no family. Ownership
checks and durable relation queries remain in their existing path. The isolated comparison retains the SQL optimization on both sides: 0.275–0.277 ms
before and 0.252–0.255 ms after. The existing manager board suite verifies the active
child overlay and parent title lookup with real SQLite; the race-enabled run passed.

## Reproduction

```sh
GOMAXPROCS=2 go test -p 1 ./internal/session -run '^$' \
  -bench '^BenchmarkManagerCatalog$' -benchtime=300ms -count=3 -benchmem
GOMAXPROCS=2 go test -p 1 ./internal/store/globaldb -run '^$' \
  -bench '^BenchmarkSessionCatalogBoard$' -benchtime=300ms -count=3 -benchmem
GOMAXPROCS=2 go test -p 1 ./internal/worktree -run '^$' \
  -bench '^BenchmarkPullRequestRepositoryIdentity$' -benchtime=300ms -count=3 -benchmem
bun web/e2e/scripts/benchmark-session-cache.ts
```

The benchmark fixtures/probe are included in this change. To reproduce a baseline,
apply only the benchmark additions to baseline `14136a511` in an isolated checkout;
run the same command without the production changes. Do not compare cold and warm
runs or overlap measurements with build/test workloads.

Local raw logs and profiles are under `.cache/performance/`: `catalog-before.txt`,
`catalog-before.mem`, `catalog-after-sort.txt`, `sql-before.txt`, `sql-plan-before.txt`,
`sql-after.txt`, `cache-before.jsonl`, `cache-after.jsonl`, `catalog-cache-{before,after}.jsonl`,
`git-identity-{before,after}.txt`, `api-identity-{before,after}.json`, and
`browser-controlled-{before,after}.json`. The browser probe and fixture inventories
are retained beside them as `browser-controlled.ts`, `lab-workspaces.json`,
`lab-seed.json`, and `live-sessions.json`; use only the isolated lab manifest and storage
state. It closes only that lab's windows through the public window-manager API, then
selects `session-sidebar-session-<id>` and waits for heading, composer and two animation
frames. Heap snapshots are sanitized and are not committed.

## Peer convergence follow-up

The complete CI on `1765f46cc` exposed a 57 ms peer task in the unchanged E2E-023
11-window envelope. A 20-repeat attribution run (`40444806962`) reproduced five
failures at 51–89 ms; restoration passed in all runs at 334.7–426.7 ms, and dragging
produced no long tasks. Six CPU-profile repetitions (`40447226708`) reproduced two
peer failures at 54 and 60 ms. These instrumented runs diagnose the path; they are
not clean performance acceptance measurements.

Samples after the window-manager snapshot update showed `DesktopShellScopedBody`,
its session catalog/attention hooks, the dock, the closed command palette, and
Base UI controls re-entering the render path. Geometry was subscribed in the parent
shell through its window-layer model, full window records, pager attention inputs,
and the closed overview's projection. React Compiler compiled these components;
the problem was their subscription boundary, not a compiler bailout.

The live window-layer adapter now owns geometry below shell chrome. The shell
selects window IDs and appearance, the pager compares the resulting attention
desktop IDs, and the overview projects windows only while open. The existing Query
cache and runtime remain authoritative; no snapshot, polling interval, gesture,
layout calculation, or event ordering is changed. Opening the overview reads the
latest projection, including changes that arrived while it was closed.

The existing OS interaction suite owns the subscription regression, alongside
window, overview and shell behavior checks: 99 focused tests pass. The closed-overview
case fails against its previous implementation. Temporary CPU and frame probes
were removed. The existing diagnostic workflow can repeat E2E-023 manually; current
uninstrumented repetitions and the full delivery result are linked from
[PR #983](https://github.com/compozy/compozy/pull/983). The 50 ms convergence and
500 ms restore budgets are unchanged. The session measurements above isolate their
named changes and predate this shell follow-up.

## Verification

| Scope | Evidence | Status |
| --- | --- | --- |
| Catalog cursor, active overlay, board, attention and keyset behavior | Focused real-SQLite Go tests with `-race -p 1 -parallel 2`, session and globaldb packages | PASS |
| Live-tail, usage, catalog reconciliation, actions and provider lifetime | Root Turbo focused run, 196 tests in five existing suites, one worker | PASS |
| Repository identity freshness, isolation and retry | Existing PullRequestService suite, real Git, race-enabled; test-shape heuristic | PASS |
| Frontend types | Root Turbo Web typecheck after building the client SDK dependency | PASS |
| Go formatting/lint | Scoped owning lane, 0 issues | PASS |
| React Doctor | Pinned 0.9.17, score 93, no new diagnostics | PASS for scanned production files |
| Production Web build | Root Turbo, repository-pinned Bun 1.4.0 | PASS |
| Shell subscriptions, window/overview behavior | Root Turbo, 99 tests in four existing suites; typecheck | PASS |
| Full Go/Web suites, generation, real daemon/browser E2E before the shell follow-up | CI `40441416318`, production head `1765f46cc` | All lanes passed except E2E-023 peer convergence at 57 ms |
| Final delivery and repeated uninstrumented E2E-023 | [Current-head PR CI](https://github.com/compozy/compozy/pull/983/checks) | CI owns acceptance; focused local checks do not replace it |
| Browser navigation and retained memory | Controlled production-browser before/after, repeated patched run, 64 active idle sessions | PASS within the measured fixture |
| Board paging, archive, family and event reconciliation | Final local daemon/Web build: 64 newly active sessions, 1,140 retained rows, 128 real rename events and 80 selections; Done pages 50 → 100 → 150 → 195, archive 3, child counts and reload | PASS |

The first CI completed every Go, Web E2E, runtime E2E and desktop E2E lane successfully.
Its Frontend lane found three action tests asserting the old `invalidateQueries`
argument shape. Those tests now assert actual cache invalidation and retain their
request/profile/rename checks; all 43 action tests pass. The behavioral cache suite
already covered the invalidation contract. This was not a production behavior failure.

The next complete CI run (`40441416318`, `1765f46cc`) passed Frontend, all Go/race
lanes, generation, Windows, runtime E2E, desktop E2E, and Web shards 1, 3 and 4.
Web shard 2 passed 70 scenarios, including E2E-006 visible-window streaming and
restore, but failed E2E-023 at 57 ms for peer convergence. Restore was 285.2 ms and
the measured drag had no long tasks. The peer-convergence follow-up above records
the resulting diagnosis and correction. Instrumented measurements are diagnostic
evidence, not final performance acceptance.

The first standalone Web typecheck failed because this new checkout lacked the client
SDK's generated build output. Building `@compozy/client` through root Turbo restored that
prerequisite; the repeat passed. The first Go lint attempt reported formatting drift in
new benchmark/test code; the scoped rerun passed after applying the repository formatter. These failed attempts are not
passing evidence. The test-convention heuristic reports four pre-existing top-level
assertion cases in the large globaldb suite; this change does not edit those cases.
The new Git-error classification initially used `errors.As`; the repository's modern
Go lint required `errors.AsType`. The production code was corrected, then both lint
tag modes and the real-Git race suite passed. The new stream-lifetime regression was
red before the dependency repair and green afterward.

`gate.sh plan` selects all `internal/store` packages plus the complete Web suite. Those
expensive lanes are assigned to CI under the user's instruction to conserve local
machine resources. No `make gate` / full local gate PASS is claimed. Delivery requires
the exact head to pass CI and real integration checks; PR #983 records that result.

The cross-surface audit is recorded in `docs/_memory/change-impact.md`. Affected QA:
`RT-visible-session-streaming`, `ET-web-sessions-board-paging-archive`, and
`ET-web-sessions-board-children`, and `ET-web-desktop-shell-lifecycle`. Existing
historical scenario verdicts do not verify this source revision.
