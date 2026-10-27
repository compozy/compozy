# Session responsiveness investigation — 2026-10-26

Status: in progress. Branch `perf/session-responsiveness`, isolated checkout
`/Users/pedronauck/Dev/compozy/_worktrees/session-responsiveness`, baseline
`14136a511` (v0.4.2). The original checkout is unchanged.

The goal is to improve responsiveness under many concurrent sessions across database
queries, frontend loading, state propagation, and memory. This report records measured
changes and the remaining investigation; the initial draft is not a completion claim.

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
bun web/e2e/scripts/benchmark-session-cache.ts
```

The benchmark fixtures/probe are included in this change. To reproduce a baseline,
apply only the benchmark additions to baseline `14136a511` in an isolated checkout;
run the same command without the production changes. Do not compare cold and warm
runs or overlap measurements with build/test workloads.

Local raw logs and profiles are under `.cache/performance/`: `catalog-before.txt`,
`catalog-before.mem`, `catalog-after-sort.txt`, `sql-before.txt`, `sql-plan-before.txt`,
`sql-after.txt`, `cache-before.jsonl`, and `cache-after.jsonl`.

## Verification and remaining work

| Scope | Evidence | Status |
| --- | --- | --- |
| Catalog cursor, active overlay, board, attention and keyset behavior | Focused real-SQLite Go tests with `-race -p 1 -parallel 2`, session and globaldb packages | PASS |
| Live-tail, usage debounce/detach and cache scope | Root Turbo focused run, 57 tests in two existing suites, one worker | PASS |
| Frontend types | Root Turbo Web typecheck after building the client SDK dependency | PASS |
| Go formatting/lint | Scoped owning lane, 0 issues | PASS |
| React Doctor | Pinned 0.9.17, score 93, no new diagnostics | PASS for scanned production files |
| Full Go/Web suites, generation, production build, real daemon/browser E2E | Current-head PR CI | Pending |
| Browser click-to-content, streaming frame/memory traces | Isolated real runtime | Pending |
| Broader loading waterfalls, retained data, other database read paths | Evidence-led audit | In progress |

The first standalone Web typecheck failed because this new checkout lacked the client
SDK's generated build output. Building `@compozy/client` through root Turbo restored that
prerequisite; the repeat passed. The first Go lint attempt reported formatting drift in
new benchmark/test code; the scoped rerun passed after applying the repository formatter. These failed attempts are not
passing evidence. The test-convention heuristic reports four pre-existing top-level
assertion cases in the large globaldb suite; this change does not edit those cases.

`gate.sh plan` selects all `internal/store` packages plus the complete Web suite. Those
expensive lanes are assigned to CI under the user's instruction to conserve local
machine resources. No `make gate` / full local gate PASS is claimed. The draft remains
in progress until its exact head passes the required CI and real integration checks.

The cross-surface audit is recorded in `docs/_memory/change-impact.md`. Affected QA:
`RT-visible-session-streaming`, `ET-web-sessions-board-paging-archive`, and
`ET-web-sessions-board-children`. Existing historical scenario verdicts do not verify
this source revision.
