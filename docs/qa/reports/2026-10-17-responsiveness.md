# Responsiveness under concurrent sessions — 2026-10-17

Status: implementation and focused local verification complete. Branch
`perf/responsiveness-audit` was created in a new worktree from `007ffb176`. The PR
delivery receipt records required CI results for its published head. No deployed
improvement or completed live-provider scenario sweep is claimed.

## Investigation and measurement boundaries

This audit covers database queries, API reads, frontend navigation/loading, live
state, retained memory, tool projection and background work. Existing improvements
from PR #784 and the attention/polling fixes are the baseline, not new gains.

A read-only observation of the operator daemon found 57 stored sessions and ten
active sessions. The installed binary was
`0.3.0-beta.34`, commit `b4636e332`, predating this branch. The sanitized local observation records
timing distributions, event counts and memory measurements without session
content or identities. Raw logs and generated benchmark output remain local and
ignored; this report records the results, methods and reproduction commands. The daemon was not restarted or modified. These
observations locate candidates; the source baseline independently confirms them.

The shared machine already had unrelated agents/tests running and load averages
above 40. They were preserved. Local checks use one coordinated Go lane
(`GOMAXPROCS=2`, `-p=1`) and at most one frontend worker. Heavy code generation,
full lint/typecheck/tests, builds and runtime/browser E2E belong to PR CI, as the
user explicitly authorized. A local focused pass is not a full gate pass.

CPU time, SQLite instructions, query counts, payload bytes, allocations and retained
heap distinguish actual work from CPU scheduling delays. Wall-clock values remain
in the raw receipts, but noisy runs are not presented as endpoint or click-latency
improvements. Synthetic fixture results do not establish production p95, INP,
current RSS, or throughput under a live provider workload.

## Changes and measured results

### 1. Task activity and facets avoid repeated history aggregation

The catalog previously aggregated run policy separately from four `UNION ALL`
passes over queued, claimed, started and finished timestamps. Activity now uses
the existing policy aggregation, with the same null/timestamp semantics. The
three catalog metadata branches share a materialized projection containing only
status and owner. Filtering, totals, facets, cursor order and page contents stay
unchanged; no schema or index is added.

A real-schema SQLite fixture has 500 tasks and 15,000 historical runs. Nine paired
samples alternate execution order after warmup. Every complete before/after
result hash matches.

| Query | CPU median before → after | SQLite VM instructions before → after |
| --- | --- | --- |
| Task catalog | 518.015 → 193.682 ms (**62.6% less**) | 13,610,019 → 4,671,154 (**65.7% less**) |
| Task inbox | 463.250 → 419.618 ms (**9.4% less**) | 10,367,311 → 8,204,209 (**20.9% less**) |

The isolated aggregation change removed 4.33 million catalog instructions; the
subsequent metadata materialization removed another 4.61 million. Combined catalog
wall median was 793.416 → 270.014 ms, but that is a synthetic SQL result, not API
latency.

A separate before/after Go benchmark uses real `OpenGlobalDB` with the same
500-task/15,000-run fixture, five iterations per sample and three samples:

| Owning Go API | Median before → after | Allocation evidence |
| --- | --- | --- |
| `ListTaskCatalog` | 1,174.919 → 402.287 ms/op (**65.76% less**) | 255,992 → 250,484 B/op; 2,148 → 2,108 allocations/op |
| `ListTaskInbox` | 979.953 → 746.545 ms/op (**23.82% less**) | 2,404 allocations/op in both variants |

The before/after timing ranges do not overlap in these samples. These are
fixture-local API calls, not HTTP or production timings. The SQL and Go timing
percentages are separate experiments and are not combined.
The reproduction commands and rejected experiments are recorded below.

### 2. Terminal approval facets use the existing active index

An equivalent active-status predicate exposes the existing partial interaction
index while preserving the exact pending-permission predicate and session-first
join plan. With 10,000 sessions and 100,000 historical interactions, sparse global
approval CPU median falls **5.422 → 0.916 ms (83.1%)**; the dense global fixture
falls **11.440 → 9.247 ms (19.2%)**. The dense narrow-workspace control remains
within noise and has no claimed speedup. Similar VM counts traverse a much smaller
B-tree; no new index, write amplification or migration is introduced.

### 3. Workspace detail hydrates only its own sessions

The workspace handler previously called `ListAll`, reading every session's
metadata and filtering afterward. The new required internal `ListForWorkspace`
capability selects catalog IDs in the requested workspace across profiles, then
uses the same metadata merge/recovery path. It retains archived/stopped sessions,
active sessions not yet published to the catalog, corrupt/missing metadata
handling, and a final scope filter. It adds no cache or alternate wire shape.

The owning integration fixture asserts full equality with `ListAll` plus the
workspace filter and reads eight metadata files instead of 32. A larger real
filesystem benchmark uses 256 sessions across 16 workspaces, with five samples
of five operations per variant and full output equality:

| Workspace detail workload | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| Metadata files read per operation | 256 | 16 | **93.75%** |
| Allocated bytes per operation | 10,270,784 | 734,072 | **92.85%** |
| Allocations per operation | 131,412 | 9,079 | **93.09%** |
| Median fixture-local elapsed time | 199.440 ms | 11.770 ms | **94.10%** |

A preliminary run that compiled during a source edit was discarded. The final
run passed with frozen source; unrelated host compilation still affects elapsed
time. The gain depends on how much history belongs to other workspaces;
this fixture's 16-of-256 reduction must not be extrapolated to a workspace where
most sessions are concentrated. Selected-session hydration and recovery checks
are deliberately retained. A separate, later read-only catalog sample contains
47 of 54 sessions in its largest workspace group; the aggregate collected at `2026-10-18T02:31:17Z`
illustrates that limitation. This is a changing workload population, not an
optimization result or the initial 57-session latency sample.

### 4. Catalog bursts preserve in-flight reads and one trailing refresh

The session catalog subscriber used to cancel and restart detail/by-ID requests
on each upsert or attention notification. Each mounted observer could restart the
same canonical read. Reconciliation now waits for existing query work, coalesces
wakes per session, and performs the necessary trailing authoritative read.
Attention-only wakes cannot discard pending catalog state. Stopped-transcript
refresh occurs only after authoritative non-live detail. Closing a window or
switching profile prevents its later follow-up without canceling shared work.

With 200 upserts and two observers, request starts fall **400 → 4 (99%)**, and
aborts **398 → 0**. The final stopped state still reaches the consumer. The
canonical catalog suite passes **20 tests**, including pending inactive prefetch,
attention interleaving, disposal, profile change and failure recovery.

### 5. Obsolete transcript arrays can be collected

The weak-key sequence trie strongly retained the projected array for every loaded
prefix. Keeping all source messages alive therefore also kept quadratic array
storage alive. A weak result reference releases obsolete arrays while active
consumers retain stable identity. Per-message conversion, order, replacement and
isolation are unchanged.

Median retained JavaScript heap across three fresh Bun processes per variant:

| Fixture | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| 2,000 messages × 1 session | 18,777,605 B | 2,047,669 B | **89.10%** |
| 4,000 messages × 1 session | 68,557,051 B | 2,623,892 B | **96.17%** |
| 1,000 messages × 8 sessions | 45,330,812 B | 7,409,657 B | **83.65%** |

The probe holds every source message and each current consumer view alive, yields
between streaming jobs and explicitly collects garbage. Output hashes match and
held-view identity remains stable. This is retained helper heap, not browser RSS.
Some elapsed runs were slower, so no CPU speedup is claimed. Frontend CI adds a
16 MiB budget for the 4,000-message fixture; its exact local command passes at
3,907,151 B. See the reproduction commands below.

### 6. Stream liveness avoids four full cache scans

Four exact Query keys now use the hash produced by QueryClient defaults. The
by-ID prefix scan remains, preserving all profile lenses; custom hash defaults
and each observer's interval rearming remain authoritative. Across 400 reschedules
with 1,006 cached queries and six observers, scans fall **2,000 → 400** with the
same 2,400 cadence evaluations. Median helper time across three processes is
**2.6948 → 0.05324 ms (98.02%)**. This excludes network and rendering.

The existing query-policy, transcript presentation and live-tail suites pass
**52 tests** via root Turborepo with one worker.

### 7. Session links open before transcript prefetch settles

Both direct and permalink routes now use the existing opportunistic preload
helper after required identity/workspace/profile validation. The canonical query
still owns history loading and errors, and the mounted consumer reuses the same
pending request. Global and sibling-profile resolution retain their scope checks.

Four controlled pending-response cases fail on baseline and pass after the change
(two routes × successful/failed transcript). All **21 router integration tests**
pass, including the 17 existing ownership/Global/profile/error cases. This proves
removal of the blocking dependency; it does not claim a measured millisecond gain.

### 8. Context reads share one event ledger snapshot

Usage, deliveries, compactions, settlement and compaction-clearing boundaries now
consume one ordered union of their five relevant event types. The same decoder
helpers and store-owner retry/cancellation behavior remain in use. This replaces
six to ten repeated reads without adding a TTL, cache, background goroutine or
public API field. A real read-only pooled database benchmark with 400 relevant
events gives these medians across five samples of five operations:

| Context workload | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| Event queries per request | 10 | 1 | **90%** |
| Event rows read per request | 801 | 400 | **50.06%** |
| Allocated bytes per operation | 9,079,920 | 7,821,243 | **13.86%** |
| Allocations per operation | 45,638 | 32,220 | **29.40%** |
| Median fixture-local elapsed time | 30.922 ms | 12.836 ms | **58.49%** |

Complete context projections match. Active/stopped/archived sessions, malformed
payload errors, first-terminal compaction boundaries, a done event without usage,
closed-handle retry, filter intersection and cursor semantics pass in the owning
race suites. The API consumes one canonical snapshot per request.
See the reproduction commands below.

### 9. Usage notifications read metadata instead of discarded blobs

The transcript stream only needs sequence, turn ID and event kind for usage
notifications. It now asks the existing session database owner for metadata.
The shared SQL builder preserves every filter, sort and limit, and all-event
cursor advancement is retained even for pages containing only text events.
Read-only pool leases and closed-recorder finalization retries remain intact.

In a 2,000-event fixture with 16 KiB payloads, a 256-event page reads **14,770 B
instead of 4,209,074 B (99.65% less projected string data)**; unused content bytes
fall **4,194,304 → 0**. A full catch-up projects 115,400 B instead of 32,883,400 B.
Metadata hashes and query plans match. A separate production Go store benchmark
reads 256 events from a 384-event fixture across three 200 ms samples:

| Go metadata read | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| Allocated bytes per operation | 4,919,958 | 165,367 | **96.64%** |
| Allocations per operation | 4,024 | 3,506 | **12.87%** |
| Unused content bytes per operation | 4,207,360 | 0 | **100%** |

The Go fixture uses a different payload envelope from the SQL probe. Elapsed
samples vary substantially, so no endpoint latency gain is claimed. Its setup
emitted a 4.847 s transaction warning outside the benchmark timer, preserved in
the raw log. The combined session/store/API race suites and the separate real
read-only pool integration pass (1.772 s for the latter).
See the reproduction commands below.

### 10. Tool projections avoid denied and unchanged work

Session policy now excludes denied tools before availability probes and schema
cloning. Unchanged HostedMCP stream digests return without cloning tool schemas.
The current versioned/unversioned cache, epoch invalidation and stream wire
contract remain authoritative. Extension availability is batched within one
projection: each instance resolves its manifest/runtime tool view once, then
reconciles all requested tools in deterministic provider order. There is no
cross-request availability cache, and actual calls still revalidate backend
availability and policy.

For a native projection with 128 tools and eight allowed by policy, allocations
fall **1,312 → 352 per operation (73.2%)** and bytes **374,275 → 225,474 (39.8%)**.
The all-128-allowed control retains the baseline **1,194 allocations and 385,243
B/op**. An initial extra decision slice regressed that control and was removed.
For an unchanged 120-tool HostedMCP stream poll, both versioned and unversioned
paths fall **530 → 169 allocations (68.1%)**. Bytes fall **108,320 → 14,880 B/op**
(versioned) and **108,323 → 14,880 B/op** (unversioned), both **86.3% less**.
The comparison uses 100 iterations and three samples; timings are noisy and are
not claimed as latency gains. Tools/MCP and the owning UDS SSE test pass with
race. The real extension subprocess fixture reduces `provide_tools`/state reads
**32 → 1 per instance** with exact parity. These are method/RPC counts, not
one total SQLite query: installation resolution remains inside both manager
operations. The fixture contains one valid runtime descriptor and 31
missing descriptors, not 32 callable tools. Both extension and daemon wrapper
suites pass with race (15.523 s and 2.307 s package runtime). An initial multi-instance fixture had an outdated
installation checksum; production correctly rejected it. The fixture now uses
the existing checksum owner, retains its assertions, and passes.
See the reproduction commands below.

### 11. Missing-origin Goal outbox rows stop generating failed deliveries

The claim query now requires the origin session to exist in the recorded
workspace before applying its batch limit. Orphan rows remain durable and
undelivered, do not starve valid rows, and become eligible again if the origin is
restored. Stopped origins and missing bound sessions remain deliverable.

The read-only operator snapshot contained 2,351 identical failures in 236.19 s
(**9.95 warnings/s**, 615,695 log bytes). In 1,000 paired SQL claims, impossible
candidates fall **1,000 → 0**. Polling remains ten times per second. This removes
repeated doomed delivery, not polling itself, and the installed daemon was not
changed. All **four real-SQLite outbox integration cases pass with race**.
See the reproduction commands below.

## Rejected and deferred candidates

- The first projection batching variant added one allocation and roughly 20 KiB
  to the all-allowed native control. The final non-batched path removes that
  storage and restores baseline allocation cost; its discarded measurements
  remain labeled in the runtime receipt.
- An additional pending-permission index did not change the plan and was rejected.
- Forcing interactions first with `CROSS JOIN` helped sparse global data but made
  dense narrow-workspace CPU 0.288 → 5.683 ms and VM steps 2,449 → 162,088. It was
  rejected; the optimizer retains join-order freedom.
- Additional scoped session recent/activity indexes stayed within timing noise.
  Existing indexes already served the bounded page; no index was added.
- The spawn reaper still performs durable session recovery checks. Replacing it
  with the in-memory list would omit accepted unbound sessions after restart and
  pending/finalizing/stopping lifecycles. The measured file-read portion alone
  did not justify that shortcut.
- Prompt-batch event decoding remains: stored payloads can be redacted/coalesced,
  so substituting the original event would change observable behavior.
- Global tool projection invalidation after calls remains. Mutating tools can
  change availability, so removing invalidation without a complete dependency
  contract would introduce stale authorization.
- Navigation-outline freshness polling remains necessary for mid-turn reply/search
  changes. No unmeasured interval change is included.
- The initial usage cursor is not moved to the transcript snapshot cursor: that
  could skip a usage event committed between independently timed snapshots. This
  change reduces read payload while preserving the existing race-safe boundary.

## Verification and delivery ledger

Each regression extends its owning canonical suite. Real SQLite and filesystem
fixtures own store/recovery behavior; real QueryClient/QueryObserver/router tests
own frontend request/state behavior. Independent static reviews cover SQL scope,
metadata lifecycle, catalog reconciliation and tool projection. None substitutes
for execution or full CI.

| Check | Current result |
| --- | --- |
| Globaldb catalog/inbox/visibility with race | Passed, 50.556 s package runtime |
| Goal outbox real SQLite with race | 4 cases passed |
| Workspace manager real filesystem with race | Passed in combined focused run |
| Context snapshot, metadata and API/SSE focused race suites | Passed: session 11.492 s, sessiondb 3.242 s, core 1.787 s; pool integration 1.772 s |
| Tools/MCP/UDS/extension subprocess/daemon suites | All focused race commands passed |
| Frontend catalog | 20 passed |
| Frontend router | 21 passed |
| Frontend query/transcript/live-tail | 52 passed |
| Exact CI retained-heap guard | Passed, 3,907,151 B / 16,777,216 B |
| Pinned React Doctor changed-file check | 0.9.17: nine changed Web files, zero errors/warnings, no skipped checks |
| Full codegen/lint/typecheck/build/test/E2E | CI-owned; exact-head outcome recorded in PR delivery receipt |

Affected scenarios have dated impact notes preserving historical evidence. Their
full historical status is not relabeled as a new pass. Cross-surface impact is
recorded once in `docs/_memory/change-impact.md`: public HTTP/UDS/CLI DTOs, native
tool IDs, hooks, configuration, persisted shapes and official `skills/compozy/`
remain unchanged; no migration or compatibility shim is needed.

The PR delivery receipt contains the published head, exact CI outcomes and
remaining measurement boundaries. The measured results and reproduction commands are versioned with
the implementation; raw logs and generated output are not tracked. No merge or deployment is part of this task.


## Reproduction commands

Run these bounded commands from the repository root, sequentially on a shared
machine. Fixtures are defined in the existing benchmark/test suites. Context,
workspace and metadata benchmarks compare both paths on the same database or
filesystem. Catalog and projection before/after measurements used the same
benchmark fixtures with production files from `007ffb176` supplied by a Go
`-overlay` map; setup, equality checks and schema migration are outside timing.

```sh
GOMAXPROCS=2 go test -p 1 ./internal/store/globaldb -run '^$' -bench '^BenchmarkGlobalDBTaskCatalogHistory$' -benchtime=5x -count=3 -benchmem
GOMAXPROCS=2 go test -p 1 ./internal/session -run '^$' -bench '^(BenchmarkManagerWorkspaceSessions|BenchmarkManagerContextEvents)$' -benchtime=5x -count=5 -benchmem
CGO_ENABLED=1 GOMAXPROCS=2 go test -p 1 ./internal/store/sessiondb -run '^$' -bench '^BenchmarkSessionDBUsageChangeRead$' -benchmem -benchtime=200ms -count=3 -timeout=90s
bun scripts/bench-session-retention.mjs web/src/systems/session/lib/session-thread-repository.ts 4000 1 16777216
```

The SQL-only experiment used Python SQLite with the repository's declarative
schema, exact owning queries, two warmups and nine paired AB/BA repetitions.
It consumed and hashed complete metadata/page results and counted VM instructions
in a separate untimed pass. The task fixture contained 500 tasks and 15,000 runs;
approvals used 10,000 sessions and 100,000 historical interactions, with sparse
(20 pending) and dense (10,000 pending) controls. Those local SQL timing results
are separate from the versioned Go API benchmarks above.

For retained-heap baseline comparison, copy the baseline repository module and
its `tool-part-error.ts` sibling into an ignored directory under `web/` so both
variants resolve identical dependencies; pass that module path to the same
script without the budget argument. Use three fresh processes per variant,
including the `2000 1` and `1000 8` workloads. The script holds source messages
and final views alive, yields between jobs, and collects garbage before/after.

Focused execution commands used for the central boundaries:

```sh
GOMAXPROCS=2 CGO_ENABLED=1 go test -race -p 1 ./internal/store/globaldb -run 'TestGlobalDBTaskCatalogPaginationAndFacets|TestGlobalDBTaskInboxUsesTwoStatementPaging|TestGlobalDBTaskCatalogCoordinatorPulseStaysInProgress|TestPageSessionsVisibilityExclusion' -count=1 -parallel=2 -timeout=4m
GOMAXPROCS=2 CGO_ENABLED=1 go test -race -p 1 '-gcflags=modernc.org/...=-d=checkptr=0' ./internal/session ./internal/store/sessiondb ./internal/api/core -run 'TestManagerListForWorkspace|TestManagerListAll|TestManagerQueriesUseWorkspaceRegistration|TestManagerEventsAndHistory|TestManagerLatestSessionEventByType|TestManagerOpenQueryRecorderValidationAndCleanup|TestCompactionMarkersAcrossConcurrentAppends|TestSessionDBQueryFilters|TestSessionDBQueryOrderedBySequence|TestOpenSessionDBReadOnly|TestSessionUsageEndpoint|TestWorkspaceHandlersDelegateToService|TestBaseHandlersWorkspaceProfileHintsTrackTeamAdoptionIT042|TestWriteUsageChangedEvents|TestTranscript' -count=1
CGO_ENABLED=1 GOMAXPROCS=2 go test -p 1 -race -gcflags='modernc.org/...=-d=checkptr=0' ./internal/tools ./internal/mcp -run '^(TestRuntimeRegistry|TestHostedService|TestHostedProjectionResponse)' -count=1
CGO_ENABLED=1 GOMAXPROCS=2 go test -p 1 -race -gcflags='modernc.org/...=-d=checkptr=0' ./internal/api/udsapi -run '^TestHostedMCPProjectionStreamGenerationCache$' -count=1
CGO_ENABLED=1 GOMAXPROCS=2 go test -p 1 -race -gcflags='modernc.org/...=-d=checkptr=0' -parallel=1 ./internal/extension -run 'TestExtensionToolProvider(Availability|Dispatch|SubprocessIntegration)$' -count=1
CGO_ENABLED=1 GOMAXPROCS=2 go test -p 1 -race -gcflags='modernc.org/...=-d=checkptr=0' -parallel=1 ./internal/daemon -run '^TestDaemonExtensionToolProvider$' -count=1
GOMAXPROCS=2 CGO_ENABLED=1 go test -p 1 -race '-gcflags=modernc.org/...=-d=checkptr=0' -tags=integration ./internal/store/globaldb -run '^TestGoalSessionOutboxIntegration$' -count=1
CGO_ENABLED=1 GOMAXPROCS=2 go test -p 1 -race -tags=integration ./internal/store/sessiondb -run '^TestReadOnlyPoolLifecycle$/^Should_preserve_navigation_reads_through_a_real_pooled_lease$' -count=1 -timeout=90s
bunx turbo run test --only --filter=./web --concurrency=1 -- --maxWorkers=1 src/systems/session/hooks/__tests__/use-session-catalog-streams.test.tsx src/routes/_app/__tests__/-agent-session-deeplink.router.integration.test.tsx src/systems/session/lib/__tests__/session-transcript-query.test.ts src/systems/session/lib/__tests__/query-options.test.ts src/systems/session/hooks/__tests__/use-session-live-tail.test.tsx
```

The final frontend command lists the same five suites executed in three separate
focused runs (20 + 21 + 52 tests), not an additional combined execution. `--only`
keeps dependency/codegen tasks out of focused diagnostics; full CI owns them.
