# SQLite performance and contention audit

## Scope

Worktree `sqlite-performance`, branch `perf/sqlite-contention`, based on
`84549c6e1fbd691611518df036d3b6165e6ce37c`. The audit covers global, session,
and workspace SQLite stores and their runtime consumers. All experiments use
isolated synthetic databases; no operator database was opened or modified.

[PR #715](https://github.com/compozy/compozy/pull/715), inspected at
`08ef98273938b56f768e8465bedfb971360bee57`, owns incremental transcript folding,
its writer cache and rollback invalidation, and duplicate redaction removal. Those changes are neither
implemented nor integrated here. Shared filenames contain separate query/open
improvements; this report does not attribute that PR's gains to this branch.
A final remote check reported #715 merged at the same inspected head. This
worktree retains the base above; it does not claim combined-branch validation.

The baseline inventory comprises 145 tables, 252 explicit indexes, and 754
static sqlc queries. Every static query was compiled with `EXPLAIN QUERY PLAN`.
Dynamic session/task/Loop catalogs, transaction lifetimes, maintenance, runtime
callers, and the standalone resource schema were reviewed separately. The [full query/table inventory](2026-10-09-sqlite-query-inventory.md) records
coverage and index decisions. Raw measurement scripts and receipts are retained
under `/Users/pedronauck/Dev/compozy/_artifacts/sqlite-performance-20261009/`.
Current gate logs and runnable Python reproductions are in `.cache/sqlite-audit/`.
Raw Go overlays stay outside the repository because the import-boundary checker
also inspects ignored Go files.

## Changes

- The shared immediate-write boundary waits for external SQLite writers with
  cancellation-aware Go timers. It preserves each connection's busy timeout and
  the existing contention budget. Ambiguous cancellation or failed rollback
  discards the connection so a pooled handle cannot expose a partial transaction.
  WAL truncation uses the same cancellation boundary and reports a busy checkpoint
  instead of claiming truncation succeeded; passive checkpoints remain best effort.
- Embedded migration bytes are validated once per immutable embedded stream.
  Mutable filesystems and live database version/refusal checks are still checked
  on every use. This removes repeated hashing and multi-megabyte allocations on
  current-database opens without weakening integrity checks.
- Session lists and agent aggregates no longer clean expired attach leases before
  reading. Read projections hide expired leases, explicit maintenance owns durable
  cleanup, and attachment uses its existing target-row compare-and-swap.
- Subagent pages use scope-specific indexed SQL; successor wake handling reads only
  its parent's pending rows. Boot recovery retains the global scan. Archive checks
  load only the archive field instead of hydrating an entire session record.
- Transcript changes read a bounded set of update-sequence groups rather than
  ranking the entire remaining history. Archive-cut checks start with entries in
  the cut range. Existing same-sequence atomicity and crossing-entry refusals remain.
- Current projection initialization and upgrades have read-only no-op paths;
  required read-modify-write upgrades recheck under immediate transactions. Token
  and hook writes use the same cancellable writer boundary.
- Scheduler claims, trigger updates, extension MCP name allocation/input replacement,
  token aggregation, and applied Loop runtime persistence use the shared immediate
  transaction owner. Coupled cursor/run, configuration, aggregate, and audit changes
  remain atomic.
- Empty global observability and terminal-file maintenance avoid unnecessary writes.
  Orphan Loop-output cleanup retains the cheap indexed generation-output check
  first, then builds the other live-reference set once instead of scanning each
  reference table for every blob. Task event list/replay queries select mutually
  exclusive indexed branches while preserving filters, order, and limits.
- Task summaries reuse the shared registry connection and omit unused audit history.
  Single-session observation recovery filters by its exact ID. Session health reuses
  its already-read previous state, and active session inventory skips projections
  that would immediately be replaced by the authoritative active snapshot.
- Profile planning closes SQL rows before filesystem inspection and removes an
  unused credential count. Lifecycle consistency checks remain in place.

## Schema and user-state preservation

The global stream adds indexes for session deletion references, subagent workspace
pages and wake messages, token/permission retention, task history/deletion, and
session-origin Goal lookup. The session stream adds an active-event sequence
index for histories with archived prefixes or suffixes. All changes are additive;
no table, column, row, unique constraint, or historical migration is rewritten.
The owning Atlas/sqlc pipeline generates the appended migrations and checksums.

Canonical reopen suites seed the previous stream prefix with session, queue,
heartbeat, permission, token, task, subagent, and archived/active transcript data.
Production reopen and a second reopen must preserve payloads, identities,
projection generation, and migration status. Existing fresh/history/integrity,
ahead-version refusal, and schema-equivalence owners remain authoritative.

## Measurements

These are isolated workload measurements on a shared Apple M4 Max, not production
p95/p99 or end-to-end UI guarantees. Side-by-side query-plan experiments compare
the same seeded data and result ordering; noisy concurrent-build samples are not
used as performance proof. The active-session and health allocation reductions
are more stable than their wall-clock samples.

| Workload | Baseline | Improved | Evidence |
| --- | ---: | ---: | --- |
| Write canceled after a 25 ms deadline under an external writer | 5.170 s | 25.535 ms | Real SQLite contention regression, default 5,000 ms busy timeout |
| Transcript change page, 10,000 entries | 38.05 ms | 0.506 ms | Existing sessiondb benchmark; complete update-sequence groups |
| Current projection reader open | 29.80 ms | 4.535 ms | Existing reader benchmark; includes immutable manifest cache |
| Session deletion, unrelated retained history | 26.697 ms | 0.052 ms | Full schema, foreign keys enabled, rollback outside timing |
| Subagent workspace page, 100,000 rows | 18.425 ms | 0.184 ms | Index-only comparison, identical scoped SQL on both sides |
| Parent pending subagents, 10 matching of 10,000 pending | 89.270 ms | 0.110 ms | Same parent identities/order; no global materialization |
| Task deletion, unrelated retained history | 15.242 ms | 0.038 ms | Full schema, foreign keys enabled |
| Session Goal projection, 100,000 completed Goals | 155.273 ms | 0.0035 ms | Scoped partial index, existing rowid tie-break preserved |
| Orphan output cleanup, 5,000 blobs / 3,000 Goal turns | 1,239.5 ms | 6.847 ms | Same 1,500 retained references; separate canonical FK-valid behavior checks |
| Task-filtered event page, 200,000 events | 12.868 ms | 0.099 ms | Final saved runner, 30 samples after 5 warmups; differential SQL parity |
| Global migration status, current stream | 3.137 ms | 0.474 ms | Same final 133-migration stream; bytes/op 3.61 MB to 16.5 KB |
| Task summary, 64 tasks / 1,024 audit rows | 22.096 ms | 3.589 ms | Identical summary JSON SHA-256 across all samples |
| Health activity allocations | 249/op | 181/op | No cache/coalescing or skipped writes |
| Active inventory allocations, 64 live sessions | about 2,974/op | 1,048/op | Metadata handling and active precedence preserved |

Cancellation-aware writes have a measured cost without contention: the smallest
write with a cancellable context increases from 38.422 to 52.225 microseconds
(+13.803 microseconds; 33 to 79 allocations). Non-cancellable contexts retain the
native path. The orphan-cleanup query also preserves its common indexed case:
with all blobs rooted in generation outputs, 1.317 ms before / 1.363 ms after.
The event-query split improves selective task/run/type pages but adds small
branch overhead to some already-indexed global replay paths: descending replay
without a cursor measured 0.053 to 0.175 ms in the final 200,000-event fixture.
The complete before/after table is retained in the raw task-event report.

The new global indexes cost approximately 1.3–2.6 MB per 100,000 rows per index in
these fixtures. No redundant parent-order replacement or speculative index on
small configuration tables was added. Existing recovery-only scans do not acquire
new indexes without a demonstrated hot-path benefit.

## Behavior and integration owners

| Invariant | Canonical owner |
| --- | --- |
| Cancellation, rollback, connection reuse, commit acknowledgement | `TestExecuteWrite` |
| WAL reader/checkpoint behavior | `TestStoreSQLiteHelpers` and SQLite recovery suites |
| Immutable caching without mutable-checksum bypass | `TestApplyRejectsAtlasSumDrift` and production migration stream suites |
| Passive session reads, expired lease projection, durable ordering | Existing global session lease/list/page/metrics suites |
| Scope, stable subagent pages, parent-only successors, restart recovery | Global subagent suites, session subagent lifecycle/recovery, daemon subagent integration |
| Transcript cursor groups, archive/rewind boundaries, projection upgrades | Existing sessiondb projection, archive, whitespace/compaction, and reopen suites |
| Scheduled claim/trigger atomicity under contention | Existing globaldb automation integration suite |
| Extension allocation and input replacement under contention | Existing MCP auth and extension environment suites |
| Token daily/session aggregation and runtime audit atomicity | Existing token-usage and Loop event/output suites |
| No-op retention and retained output references | Existing observability, terminal journal, and Loop output suites |
| Summary equality and health/lifecycle behavior | Existing observe integration and session health/lifecycle suites |
| Profile planning and lifecycle behavior | Existing complete profile race suite |

## Verification status

Completed focused runs use real SQLite and race detection: the full sessiondb
unit/integration suite, full workspace/profile suites, full observe integration
suite, global session/subagent owners, shared writer/checkpoint/integrity owners,
and session/daemon subagent lifecycle and restart recovery. Independent core and
query reviews found no correctness findings; 447 initial differential SQL cases
and 1,074 additional final orphan-query comparisons preserved results, scope,
ordering, and null behavior. Full generation and `mise exec -- go build ./...` passed. The test-convention
comparison across 25 existing edited suites found zero new heuristic findings
(41 inherited findings remain unchanged).

The final combined non-session globaldb race/integration owners passed in 29.107s;
full workspace and profile race suites passed in 2.438s and 24.277s. All 144
historical migration files and their existing Atlas checksum entries are unchanged.

The canonical fresh/reopen/ahead owner passed for all three migration streams
in 135.339s with race and the repository gate's modernc checkptr flags.
The required pre-rebase `make gate` did not pass. Code generation and both lint
configurations passed, as did every selected package except `globaldb`. That
package reached its 45-minute limit; six historical migration subtests reported
context deadlines, and the marketplace source migration was still running when
the package alarm fired. No race or data-equality failure was reported. The host
had observed load averages of 64–112 with 16 logical CPUs during the late phase;
that resource contention is evidence, not proof that every failure is environmental.
The complete failed run is retained in `.cache/sqlite-audit/gate-final.log` and
`.cache/gate/logs/go-test-1791611767-28033.log`. Full gate validation remains pending.
An unchanged `TestGlobalDBSessionAttentionMigration` rerun with one test worker
also exceeded its existing five-minute upgrade context at migration 111, before
the preservation assertions (308.856s package duration). Host load remained above
100. Its log is `.cache/sqlite-audit/session-attention-isolated.log`. The fixture
also retains a pre-upgrade context across reopen, but that lifetime issue alone
does not explain the isolated replay timeout; neither result is counted as passing.
On October 10, the user directed heavy/global validation to CI and authorized
pushes for remediation. The resumed local gate passed code generation, then was
stopped during lint at that direction, before starting another global test run.
The checkpoint and rebase proceed under that explicit delivery override; complete
validation will be reported from CI for the final branch head.
A parallel historical migration test reused its replay-phase context for a separate
ahead-version fixture after that context expired. That fixture phase now receives
its own existing-budget context; replay, equivalence, integrity, and refusal
assertions and timeout values are unchanged. A diagnostic rerun then exceeded
the replay budget under modernc race/checkptr instrumentation; its CPU profile
and failed log are retained. The passing final run uses the repository gate's existing
modernc checkptr flags and unchanged deadlines. Neither failing run is counted
as passing evidence.
Earlier attempts during concurrent edits with missing generated methods or an
in-progress checksum file are likewise not verification evidence.

## Remaining limits

- This audit does not change the one-writer-per-file SQLite model or the public
  unbounded `ListAll` contract. Large stopped-session inventories still scale with
  retained metadata and ownership checks; paginated catalog paths remain bounded.
- Profile Rename/Delete still hold the writer while callbacks inspect external
  filesystem state. Moving those callbacks alone would weaken current SQL-backed
  stale-plan fencing; the audit records this residual instead of making that change.
- Hook observation still opens a session store per operation. Reusing writable
  handles needs an owner compatible with session-family deletion/replacement;
  no speculative second lifetime manager was introduced.
- Some cold, write-first configuration transactions still use native SQLite busy
  waits. Hot reproduced paths were moved to the shared boundary. Direct external
  writers can still contend, but cancellation-aware admission bounds affected
  operations and does not misreport a committed write as canceled.
- Schema indexes improve demonstrated query plans; they do not establish a general
  production latency SLA. Browser journeys and external-provider performance must
  not be inferred from store benchmarks. PR #715 remains an independent change.
