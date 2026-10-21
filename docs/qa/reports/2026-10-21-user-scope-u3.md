# U3 User execution slice evidence

Scope: USW step 4, BR-1/BR-8, T-11 through T-16 and the terminal-journal portion of T-28.
This is worker verification, not a full transport/browser QA verdict.

## Implemented boundaries

- User Loop runs use an empty workspace with an explicit profile. Inventory queries retain exact
  workspace equality, including all-profile reads; request payloads expose `profile_id`.
- User configuration, executed snapshots, UI annotations, admission keys and queue promotion are
  profile-scoped. Project sidecars retain the empty profile key.
- Watch delivery compares exact workspace identities. Goal checkpoint, binding and outbox keys
  accept User runs; the canonical daemon command suite starts and projects a User Goal.
- User terminals resolve profile configuration without project registration. Journal commands,
  recordings and artifacts use the reserved `user-<profile_id>` workspace database directory.
  Profile deletion journals terminal closure/removal and supports lifecycle retry.
- Heartbeat accepts an eligible User session.

## Verification

Passed:

- `make codegen` (SQL bindings, migration, OpenAPI, TypeScript/Go clients, CLI documentation).
- `go build ./...`.
- Focused `go vet` for core/contract, daemon, heartbeat, loop, profile, store/globaldb/workspacedb,
  terminal and journal packages.
- `TestGlobalDBLoopUserScope`: real SQLite config/snapshot/annotation/admission isolation, exact User
  run and request inventories, owner labels, and promotion without changing the other profile's queue.
- Existing project run inventory and concurrent admission regression suites.
- Race detector on User isolation/queue and concurrent admission suites (`go test -race`).
- `TestLoopWatchEventsObserverShouldMatchAndWake`, including project/User separation.
- Canonical Goal command lifecycle, including User start/projection, and Goal key/outbox contracts.
- Terminal manager User open/profile fence/reconnect and real journal record/search/artifact/removal.
- Profile deletion journal failure followed by `RetryOp`, then idempotent recovery.
- Eligible User heartbeat and shared HTTP/UDS User route guard tests.
- `TestProductionMigrationStreamsFreshReopenAndAhead`, including schema equivalence.
- With `-tags integration`: Loop configure/executed-snapshot persistence and Goal session outbox suites.
- Changed-file formatting, generated TypeScript lint and `git diff --check`.

No `make gate`, full Go test suite or E2E run was executed, per the slice brief.

## Integration seams

- U2: the only session-domain edit permits a creation profile with empty workspace when its profile
  is explicit. Reconcile that validation with U2's session runtime implementation. The User Goal test
  uses this seam; full session creation and WM remain U2-owned.
- U4: `00145_schema.sql` only rebuilds the four sidecar tables; legacy pseudo-workspace normalization,
  per-profile copying, aliases and global task/automation repointing remain U4-owned. Renumber the
  generated migration if another slice generated the same number.
- Profile recovery now runs after terminal initialization, so pending User journal removal can execute.
- User journal reads require a concrete profile. Live terminal inventories support owner-labeled
  aggregate reads; a cross-profile journal merge is not introduced by this slice.
- Integrated Web/CLI QA and official skill examples must be reconciled with the remaining slices.
  Scenario additions are recorded in the Loop lifecycle, Goal command and terminal profile scenarios.
