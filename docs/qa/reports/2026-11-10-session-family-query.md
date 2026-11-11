# Statistics-free session family traversal

## Reproduction and result

The recursive `ReadSessionWorkFamily` step could choose a profile/workspace catalog index
instead of the parent edge index when SQLite had no statistics. A fresh migrated fixture
initially chose the good plan. Recreating the existing `idx_sessions_profile_catalog_recent`
with its identical DDL made the old query choose that index instead. This reproduces the
index-order sensitivity without changing the final schema or inventing planner statistics.

The canonical catalog suite now captures the SQL executed by sqlc, verifies that `sqlite_stat1`
is absent, checks all 840 returned members across 120 seven-member trees, and explains that
same statement. Before the fix the parent-index assertion fails; afterwards the recursive
step is `SEARCH s USING INDEX idx_sessions_parent (parent_session_id=?)`.

`INDEXED BY idx_sessions_parent` pins this selective traversal independently of statistics
and index creation order. Existing workspace/profile and fork/continue predicates remain.
There is no schema change or new migration.

## Measurements

Darwin arm64, Apple M4 Max, Go benchmark using the repository's SQLite driver. Each sample
uses the same temporary fixture, 120 requested roots, 840 sessions, three levels per tree,
no statistics, and the recreated catalog index. Maintenance is stopped for this fixture so
statistics cannot appear during the measurement. The benchmark checks the 840-row result
on every iteration. Command (five samples, one-second target per sample):

```sh
go test ./internal/store/globaldb -run '^$' -bench '^BenchmarkSessionWorkFamily$' -benchtime=1s -count=5
```

| Sample | Original query (ms/op) | Parent-index query (ms/op) |
| --- | ---: | ---: |
| 1 | 468.732278 | 4.212737 |
| 2 | 603.208562 | 2.996573 |
| 3 | 582.191166 | 2.873189 |
| 4 | 528.353750 | 3.061415 |
| 5 | 568.049604 | 2.855907 |
| Mean | 550.107072 | 3.199964 |

The candidate benchmark was run alongside the catalog regression tests; its fixture and
measurement loop are identical. These sequential before/after runs share a development host,
not a dedicated performance runner. The approximately 172-fold mean reduction describes
this synthetic query workload, not route latency or overall daemon CPU. An initial fresh-index
control already chose the parent index and measured 2.97–3.39 ms/op before the fix.

## Batching and maintenance

The current `activeBoardSessionCatalogRows` already passes the complete visible session batch
to `SessionLinkedWork`, which calls `SessionWorkFamily` once. The existing projection suite now
asserts one family call containing all roots across multiple scopes. No additional batching
abstraction or public read model was needed.

The existing core/telemetry checkpoint worker also runs statistics maintenance on its first
one-second tick and hourly after success. Failed attempts back off for one minute while
checkpoints continue every second. It uses
`PRAGMA optimize = 0x10012`: consider all tables because this dedicated connection has no
application-query history, keep SQLite's bounded analysis flag, and request statistics refresh.
A 100 ms context deadline bounds interruptible execution; the maintenance connection retains
`busy_timeout(0)`. Kernel I/O itself is not a hard realtime deadline guarantee. See the
[SQLite optimize reference](https://www.sqlite.org/pragma.html#pragma_optimize).

Real SQLite checks verify that optimization yields to an occupied writer, later creates index
statistics through the periodic worker, preserves all rows, and honors cancellation. Existing
checkpoint and shutdown tests also pass. Telemetry already registers a PASSIVE checkpointer;
the reported large WAL is not established to be a missing checkpoint. A separate investigation
must distinguish retained allocation, pinned readers, backfill and reuse. This change makes
no claim about the reported telemetry WAL size or RSS.

## Verification and boundaries

Passed:

- `go test ./internal/store -run '^TestStoreSQLiteHelpers$' -count=1`
- `go test ./internal/store/globaldb -run '^TestSessionLinkedWorkCatalogReview$' -count=5`
- `go test ./internal/session -run '^(TestManagerBoardCatalog|TestSessionLinkedWorkProjection)$' -count=1`
- `go test ./internal/api/core -run '^(TestBaseHandlersSessionAttentionSurfaces|TestBaseHandlersAttentionOperatorScope|TestSessionOptionalHealthLinkedWork)$' -count=1`
- `COMPOZY_GO_LINT_SCOPES='./internal/store ./internal/store/globaldb ./internal/session' COMPOZY_GO_LINT_CONCURRENCY=4 make go-lint` (normal and integration tags, zero findings).
- `make codegen-check` after repairing the local dependency installation with `bun install --frozen-lockfile --ignore-scripts`. The first `make codegen` regenerated SQL but later failed because the installed SDK generator lacked the repository's patch; the frozen reinstall and full check resolved this without source/dependency changes.
- `git diff --check`.

The test-shape heuristic passes the changed store suite. Its findings in the catalog/session
files concern pre-existing, untouched top-level tests; the added/extended cases use the existing
canonical subtests. No tests or assertions were weakened. Local race/full suites and E2E were
not run, as required by the dispatch; current-head CI owns those gates. Real SQLite and read-model
runs are the runtime evidence here. No live provider/browser latency or operator database access
is claimed. The affected scenario is `ET-session-pull-request-link`.
