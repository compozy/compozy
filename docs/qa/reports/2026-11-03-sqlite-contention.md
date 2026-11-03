# SQLite contention isolation — issue #1227

Status: implementation delivered to the branch; focused verification and PR delivery are in progress.
The operator authorized shipping the safe implementation for this release with remaining measurements
listed explicitly. This report does not claim that the 30-session performance acceptance has passed.

The implementation baseline is `d35c63c9a`. Main through `c96f54f3e` was merged before the release push (merge `c8c420c31`).
All work is isolated in `fix/issue-1227-sqlite-contention`. Operator runtime databases, credentials
and running sessions were not changed. The incident log was read only for sanitized aggregates.

## Measured incident holders

These counts cover the complete incident log, not only the narrower 16:00–16:35 window.

| Operation | Holds over one second | Sum of reported holds | Maximum | Median |
| --- | ---: | ---: | ---: | ---: |
| Transcript fold write | 45 | 108,193 ms | 9,048 ms | 1,888 ms |
| Event-summary write | 8 | 22,981 ms | 7,265 ms | 2,195 ms |
| Observability retention | 1 | 3,711 ms | 3,711 ms | 3,711 ms |

Health and model-catalog writes each reported 11 exhausted waits. Seven of the 22 failures named
event-summary writes as the owner; one named health. The other 14 reported `owner_known=false`.
All 14 had `attempts=0`; 13 had `wait_ms=0`, and one had `wait_ms=15`. They failed before a SQLite
write attempt, so the log does not establish an external lock holder. Cancellation before admission
or while another admitted writer is acquiring its connection is also consistent with these records.
The exact historical owner cannot be recovered from this evidence.

## Implemented changes

- Interactive writers take precedence over queued background work. The bounded background queue
  coalesces up to 64 operations, isolates each with a savepoint, and acknowledges each outcome after
  commit. It yields after an item when interactive work is waiting or the 10 ms batch budget has
  elapsed. An individual atomic operation is not preempted. Mutation authorization fences retain
  an individual commit. Cancellation, reentry rejection and shutdown join remain explicit.
- Event summaries, model-catalog updates, process observations, token totals/daily rollups,
  permission audit writes and heartbeat wake decisions use the background owner. Each producer
  remains visible in contention diagnostics while its callback owns the transaction.
- Health stores the latest dirty observation per session, serves it immediately, flushes every
  500 ms and drains on clean close. Failed flushes retain dirty values. Deleted or moved session
  ownership invalidates stale observations. A crash may lose an unflushed health observation;
  clean shutdown reports flush failures rather than claiming persistence.
- Request reads in global repositories and the profile manager use a separate `mode=ro`,
  `query_only` pool. Transcript reads and preparation use a separate guarded session pool.
- Transcript event assignment, fold reduction and JSON encoding run in a read snapshot. The short
  publication transaction validates projection state and the next sequence before committing the
  canonical ledger and projection together. Concurrent append/archive/clear/rewind invalidates
  preparation. Speculative cached folds are published only after a successful commit.
- Event summaries move to `compozy.db.telemetry.db`, with its own writer, reader and migration
  stream. The core writer never attaches this file. Core read connections attach it read-only and
  install a scoped view, retaining Observe responses, metadata, filters and cursors.
- Retention removes at most 256 rows per table per item. Core retention skips its write transaction
  when its read probe finds no expired rows. Profile/workspace deletion records a durable tombstone
  that hides telemetry immediately, then retries bounded physical cleanup. Profile deletion completes its recoverable lifecycle only
  after telemetry cleanup succeeds; a failure remains visible and retryable.
- MCP retirement and worktree terminal transitions retain atomic state/event durability through a
  core outbox. Failed delivery does not turn a committed state transition into an apparent rollback.
  Delivery is idempotent, verifies the full payload, and acknowledges the outbox only after the
  telemetry commit. Startup, retention and later lifecycle deliveries retry pending entries.
- The incident's direct `ClearLaterReturn` mutation now uses named interactive admission. Process
  observation and heartbeat decision transactions no longer bypass admission with raw `BeginTx`.

## Upgrade and compatibility

The published migration through main's `00185_schema.sql` and every prior checksum are unchanged.
The branch's unpublished relocation migrations follow it at versions 186–189. The migration owner
freezes the old summary ledger, copies 128 rows per batch, verifies every column and sequence, and
commits the destination with `synchronous=FULL` before deleting each source batch. An interrupted
acknowledgement resumes without duplicate events. A conflicting destination preserves the source
and fails explicitly. The high-water cursor survives even when retention deleted every source row.

Startup refuses a missing relocated file. Backups and restores must preserve both databases and
all sidecars. The old core file can retain reusable free pages; this change does not run a large
blocking VACUUM during startup. Operator database/file-location docs and the official runtime
reference describe the sibling file and recovery boundary. Status reports both migration streams.

Observe's wire shape, native tools, CLI commands, configuration, hooks and extension SDKs remain
unchanged. `schema_streams` gains the telemetry entry in its existing array. Authoritative task
and session ledgers remain with their owning state; heartbeat decisions and permission audits
retain their existing durable semantics. No storage dependency or CGO requirement was added.

## Index audit

The old ledger had 14 declared indexes plus its unique-ID index. The owning query inventory is
`global_db_observe.go`, `global_db_observe_overview_scoped.go`, `global_db_watch_events_observe.go`
and `queries/observe*.sql`.

- Remove hook-event, parent-session, root-session and workflow indexes: these fields are selected
  as correlation metadata, but the current summary readers do not filter on them.
- Replace the full outcome/timestamp index with a partial failure/warning timestamp index. The
  fixed error-only predicate uses the matching SQL literals. Other outcome filters retain their
  normal scoped/time access paths; no outcome-specific latency improvement is claimed.
- Keep provider/time, timestamp, type, session, workspace, worktree, task, run and actor indexes
  for their current filters and retention. Add profile/time and profile/sequence for scoped reads.
- Avoid a redundant workspace/sequence index: the existing workspace index already orders its
  entries by the integer primary-key sequence. The final telemetry schema has 12 declared indexes
  plus unique ID, compared with the previous 14 plus unique ID.
- This is a source-based audit, not a measured production query-plan or index-size comparison.
  The release follow-up must measure selectivity, page count and p99 on representative retained data.

## Writer ownership inventory and limits

| Source | Finding and disposition |
| --- | --- |
| Daemon observer | `observer_factory.go` injects the daemon registry; it does not open a second global writer. |
| Standalone observer | `observe.New` can open another handle when no registry is injected. Its summaries now target telemetry; other core mutations still use SQLite's cross-handle locking. |
| Process observations | Raw transaction replaced with named, ordered background execution. |
| Heartbeat wake decision | Event/state atomic transaction replaced with named background execution. |
| Token/permission producers | Direct generated mutations routed through the background owner. |
| Return from Later | Direct mutation observed in the incident routed through interactive admission. |
| Extension CLI | Uses daemon APIs while the daemon runs; stopped-daemon local listing opens the database directly. |
| Provider-auth metadata | Can open an independent handle; metadata itself is read-only, but normal open/migration/recovery still applies. |
| Vault-key migration | Opens independently and uses its own migration file lock, not the daemon's admission gate. |
| Remaining direct writers | Marketplace/profile-attention replacements, some generated one-statement mutations and extension-registry operations remain outside this process-local admission owner. |

The admission gate is still keyed by `*sql.DB`; it does not order other handles or processes.
Those remaining paths are not proven unable to conflict. A shared-file/process ordering change
and complete attribution require a separate measured follow-up; this release does not claim to
close that acceptance criterion. No speculative runtime SQL classifier or process-wide storage
wrapper was introduced for the release.

## Verification receipts

Before the host-load restriction, focused write, global read-isolation, health, catalog, process,
transcript/replay and lifecycle checks passed. Focused store/globaldb/sessiondb lint had zero issues.
Those receipts belong to checkpoints `cd0b7b1a9` and `e7345bd91`, not final-head CI.

After the operator restricted local execution, checks run one function at a time with
`GOMAXPROCS=2 go test -p 1 -parallel 1 ... -count=1 -timeout=120s`:

| Owning function | Result |
| --- | --- |
| `store.TestProductionMigrationStreamsFreshReopenAndAhead` | Passed (11.185 s): all four streams; fresh/reopen, integrity, schema equivalence and ahead refusal. The initial attempt found the new stream's baseline filename mismatch; production migration naming was corrected and the unchanged assertion passed. |
| `globaldb.TestGlobalDBTelemetryRelocation` | Passed (0.912 s) after final cleanup checks: all-column preservation, failed acknowledgement/restart, retired cursor, missing-file refusal and conflicting destination. |
| `globaldb.TestGlobalDBTelemetryDelivery` | Passed (0.735 s): full core writer occupation, second read connection, read-only enforcement, workspace tombstone/cleanup, durable outbox failure/replay. |
| `store.TestBackgroundWrites` | Passed (0.102 s) after queue deadline classification and per-producer attribution. |
| `daemon.TestDaemonToolEventSink` | Passed (0.712 s): a completed tool event survives a real telemetry writer lock and a canceled persistence attempt. The first run exposed a test seed copying only the core file; the shared seed now copies the full database family. |
| `store.TestProductionMigrationStreamsFreshReopenAndAhead/Should_fresh-apply,_reopen,_and_reject_an_ahead_telemetry_stream` | Passed (0.079 s) after the final index migration. |
| `profile.TestManagerProfileLifecycle/Should_preview_and_sweep_the_complete_delete_catalog` | Passed (0.287 s): preview and deletion retain the existing full ownership assertions. |
| `profile.TestManagerRecoveryAndReservation/Should_retry_telemetry_removal_before_completing_profile_deletion` | Passed (0.242 s): an actual SQLite delete failure leaves a hidden profile and retryable lifecycle; retry removes the retained telemetry before completion. |
| `daemon.TestBootInitializesRuntimeAssemblerIntegration` | Passed (1.013 s): real isolated daemon boot and shutdown; both persisted migration streams are reported. |
| `globaldb.TestGlobalDBHeartbeatWakeAuditStore` | Passed (0.810 s): durable event/state decision behavior remains intact. |
| `globaldb.TestGlobalDBWorktreeStore/Should_commit_an_exit_terminal_state_and_replay_event_atomically` | Passed (0.541 s) before main integration; CI reruns the integrated suite. |

The owning Atlas and sqlc generators completed after main integration. Full `make codegen` passed
before integration; `make codegen-check` was externally terminated and is not a passing receipt.
The test-convention heuristic reported existing legacy test shapes and file-wide parallel/environment
false positives; the new recovery and telemetry cases use the canonical subtest suites and check cleanup
errors. Formatting was restricted to the changed Go files. No local full/race/E2E suite or repository-wide lint/gate was run. Final formatting, lint, codegen,
build, unit and E2E delivery evidence belongs to CI on the PR head.

## Explicit release follow-up

The operator authorized deferring work that cannot safely fit the release window. The following
remain unverified and are not implied by the focused tests:

1. Same-machine before/after load with approximately 30 streaming sessions: p99 transcript/catalog
   latency, transactions/sec, core bytes/minute, zero interactive failures and no holds over one second.
2. A production-sized (~1 GB) upgrade, representative index page counts/query plans and startup time.
3. An isolated real-daemon multi-session QA lab and the complete `MS-daemon-host-load-governance`
   journey. No operator provider credentials or runtime home were reused.
4. Complete same-file/multiple-handle/direct-writer attribution and ordering. Large individual
   catalog/process operations retain atomicity and need their maximum hold measured.
5. Final-head CI plus two CodeRabbit reviews, followed by merge-train handoff; this agent does not merge.
