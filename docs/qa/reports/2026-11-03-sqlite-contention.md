# SQLite contention isolation

Status: **in progress**. This report is not acceptance of issue #1227.

The baseline is `d35c63c9a`, on the same development machine used for the isolated
checks. Operator databases, credentials and running daemon state are untouched.

## Current implementation

The local admission gate gives queued interactive writers precedence over background
batches. A bounded background queue groups items in immediate transactions, isolates
callback failures with savepoints, and returns results after commit. It yields between
items when an interactive writer arrives or the batch time budget is reached. Waiting
cancellation, reentry detection and commit outcome identity are preserved. Authenticated
mutation fences receive an individual transaction, so unrelated work cannot extend the
interval between authorization and commit. Event summaries, model catalog refreshes and
process observations now use this owner. Session health retains the latest dirty value,
serves it immediately, flushes periodically in batches and drains at clean close. Failed
flushes retain dirty values for retry. Session deletion/movement invalidates stale health.

The global database opens an independent `mode=ro`, `query_only` pool. Repository read
queries and read transactions use this pool; mutations keep their writer handle. The
SQL remains owned by the existing sqlc catalogs, including profile/workspace filters.
Migration-prefix fixtures explicitly inject their historical handle into both roles;
production opens distinct handles.

Session appends now prepare assignment, event reduction and JSON in a read snapshot.
The write transaction validates the projection generation/state and next event sequence
before publishing the event ledger and prepared projection atomically. A competing append,
archive, clear or rewind invalidates preparation and causes a fresh snapshot. Session
request reads use a separate guarded read-only pool.

## Observed lock holders

Read-only analysis of the operator's incident log produced these sanitized aggregates.
These cover the complete supplied log, not only the incident's narrower time window.
No runtime database or credentials were read or changed.

| Operation | Long-holder warnings | Total reported hold | Maximum | Median |
| --- | ---: | ---: | ---: | ---: |
| Transcript fold write | 45 | 108,193 ms | 9,048 ms | 1,888 ms |
| Write event summaries | 8 | 22,981 ms | 7,265 ms | 2,195 ms |
| Sweep observability | 1 | 3,711 ms | 3,711 ms | 3,711 ms |

Health upserts and model catalog writes each exhausted contention 11 times. Seven
health failures named event-summary writes as the owner. Across both operations, 14
failures reported an unknown owner. This identifies the main measured holders and
their victims; it does not establish the identity of those unknown owners. Transcript
preparation and telemetry write/retention isolation are therefore required, beyond
the shared admission queue.

## Focused evidence

- `GOMAXPROCS=4 go test -p 2 ./internal/store -run 'Test(ExecuteWrite|BackgroundWrites)$' -count=1 -timeout=90s`
  passed. The existing write suite now covers batching, failure isolation, interactive
  priority, waiting cancellation, reentry, shutdown draining, mutation fences and
  cancellation racing a successful commit.
- Global open/reopen/composition suites passed after the initial read-pool conversion.
- The new global open subtest `Should keep request reads independent of an exhausted
  writer pool` passed with a real SQLite immediate transaction occupying the sole writer
  connection. Workspace, catalog and health reads still completed; the reader refused
  a write. The fixture supplies the catalog's required sort explicitly.
- Focused `golangci-lint` for `internal/store` and `internal/store/globaldb` passed after
  the helper-read audit with zero issues. Focused workspace CRUD, session health, catalog
  visibility and stable-keyset suites also passed.
- The write test-shape checker passed. The global open suite checker reports existing
  findings; comparison against the baseline confirmed that the new subtest adds none.
- Focused lint for store/globaldb/sessiondb passed with zero issues after the producer and
  transcript changes. No full test suite, race suite, E2E or repository-wide gate has run locally.
- Focused health, model catalog, event-summary atomicity and process-observation suites
  passed after producer integration. Health's canonical suite now verifies updates and
  reads with the sole writer occupied, latest-value persistence across reopen, and retry
  after a real SQLite trigger rejects a flush.
- Transcript projection/replay, append-if-absent, chunk coalescing and close/reopen suites
  passed after moving reduction outside the transaction. Competing-handle cases
  for append/archive/clear invalidation also passed (1.715s for the focused session suite).
- `make codegen` regenerated store queries but failed later because the locally installed
  TypeScript SDK generator lacks the repository's patched stream-cancellation template.
  An isolated `bun install --frozen-lockfile` repaired the installation and the next
  `make codegen` completed. `make codegen-check` reached dependency verification but was
  terminated by signal 15; it has not passed and remains a required check.

## Remaining acceptance work

- Connect and validate background producers, coalesced ephemeral state and clean-shutdown
  persistence; bound expensive individual background operations.
- Move telemetry out of the core file, audit its indexes and ship a crash-safe, lossless
  migration, including preserved Observe scopes and profile metadata. Complete codegen
  and owning upgrade/reopen/schema-equivalence checks.
- Finish transcript preparation invalidation coverage and measure actual hold times.
- Complete writer ownership inventory and ordering across handles and direct SQL paths.
  Source inspection found direct generated mutations and several `BeginTx` writers that
  bypass admission, including process observations. This identifies possible in-process
  unknown owners, not the exact owner in the historical incident.
- Measure the reproducible ~30-session baseline and result: p99 transcript/catalog reads,
  transactions/sec, global bytes/minute, interactive errors and long transactions.
- Run the isolated real daemon journey, update this report with actual receipts, and open
  the PR using the repository template. Current-head CI and reviews remain delivery gates.
