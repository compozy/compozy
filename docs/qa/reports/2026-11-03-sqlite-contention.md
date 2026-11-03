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
interval between authorization and commit. Producers are not connected yet.

The global database opens an independent `mode=ro`, `query_only` pool. Repository read
queries and read transactions use this pool; mutations keep their writer handle. The
SQL remains owned by the existing sqlc catalogs, including profile/workspace filters.
Migration-prefix fixtures explicitly inject their historical handle into both roles;
production opens distinct handles.

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
- No full test suite, race suite, E2E or repository-wide gate has run locally.

## Remaining acceptance work

- Connect and validate background producers, coalesced ephemeral state and clean-shutdown
  persistence; bound expensive individual background operations.
- Move telemetry out of the core file, audit its indexes and ship a crash-safe, lossless
  migration, including preserved Observe scopes and profile metadata. Complete codegen
  and owning upgrade/reopen/schema-equivalence checks.
- Move transcript reduction outside its write transaction with validated publication.
- Complete writer ownership inventory and ordering across handles and direct SQL paths.
  Source inspection found direct generated mutations and several `BeginTx` writers that
  bypass admission, including process observations. This identifies possible in-process
  unknown owners, not the exact owner in the historical incident.
- Measure the reproducible ~30-session baseline and result: p99 transcript/catalog reads,
  transactions/sec, global bytes/minute, interactive errors and long transactions.
- Run the isolated real daemon journey, update this report with actual receipts, and open
  the PR using the repository template. Current-head CI and reviews remain delivery gates.
