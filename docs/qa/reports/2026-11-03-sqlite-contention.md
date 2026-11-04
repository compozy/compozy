# SQLite contention isolation — issue #1227

Status: implementation and verification are in progress. The operator reaffirmed the full issue scope:
the remaining ownership work, representative upgrade and 30-session measurements are required before
completion. Focused correctness checks do not establish the performance acceptance criteria.

The implementation baseline is `d35c63c9a`. Main through `dc37fd7f0` is integrated. Its version 186 remains byte-for-byte intact;
the unreleased branch migration tail was resequenced after it during conflict resolution.
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
  If SQLite aborts the whole transaction during a failed item, rollback/discard completes before
  healthy peers retry in individual transactions. This recovery never replays an ambiguous commit.
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
- Catalog hydration, normalization and JSON content comparison run before writer admission. A
  durable metadata revision fences prepared comparisons across handles/processes. A concurrent
  publication or earlier source pruning in the same batch forces replacement, preserving atomic
  generations without holding the writer for content comparison.
- Event summaries move to `compozy.db.telemetry.db`, with its own writer, reader and migration
  stream. The core writer never attaches this file. Core read connections attach it read-only and
  install a scoped view, retaining Observe responses, metadata, filters and cursors.
- Retention removes at most 64 telemetry rows (256 for core tables) per item. Core retention skips its write transaction
  when its read probe finds no expired rows. Profile/workspace deletion records a durable tombstone
  that hides telemetry immediately, then retries bounded physical cleanup. Profile deletion completes its recoverable lifecycle only
  after telemetry cleanup succeeds; a failure remains visible and retryable.
- MCP retirement, worktree terminal transitions and suggestion dismissal retain atomic state/event
  durability through a core outbox. Failed delivery does not turn a committed state transition into an apparent rollback.
  Delivery is idempotent, verifies the full payload, and acknowledges the outbox only after the
  telemetry commit. A foreground operation publishes its own committed event; backlog drains are
  bounded to 128 items and isolate failures per item. An owned one-second retry loop handles
  deferred delivery without depending on the daily retention sweep, and clean close joins it and
  drains the remainder. Startup logs a deferred outbox failure while preserving core availability.
  Observe readers await publication attempts already in flight without reserving SQL connections;
  subsequent attempts cannot extend that wait. Failed attempts remain replayable in the outbox.
  Replay cursors are assigned only by the telemetry ledger and never change during delivery.
- The incident's direct `ClearLaterReturn` mutation now uses named interactive admission. Process
  observation and heartbeat decision transactions no longer bypass admission with raw `BeginTx`.

## Upgrade and compatibility

The published migration through main's `00186_schema.sql` and every prior checksum are unchanged.
The branch's unpublished relocation migrations follow it at versions 187–190. The migration owner
freezes the old summary ledger, copies 128 rows per batch, verifies every column and sequence, and
commits the destination with `synchronous=FULL` before deleting each source batch. An interrupted
acknowledgement resumes without duplicate events. A conflicting destination preserves the source
and fails explicitly. The high-water cursor survives even when retention deleted every source row.

Before freezing the source, the coordinator commits a random family identity in core metadata
and the auxiliary identity table (telemetry migration 3). A crash between these commits resumes
before export. Every acknowledged batch and every relocated reopen verifies that identity.
Startup refuses missing, uninitialized, unbound or mismatched destinations as soon as export can
acknowledge source rows, including an interrupted version-188 upgrade. A schema-valid empty
replacement and a file from another core cannot silently replace already acknowledged events. Older home-workspace upgrades read their migration
inventory through the relocated read owner. Backups and restores must preserve both databases and
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
- The 830,020-row paired fixture measures 1,069,608,960 bytes of table/index pages before relocation
  and 1,059,639,296 afterward with the final ordering indexes; the table itself remains 681,644,032 bytes. Removing unused indexes
  is partly offset by the two profile indexes. The full outcome index uses 44,470,272 bytes before
  replacement, versus 4,096,000 for the partial index. These are synthetic retained-data measurements,
  not operator-database measurements.
- The initial query plans exposed an ordering defect: `ErrorOnly` still scanned the full timestamp
  index, and profile-recent reads needed a temporary sort for sequence ties. Telemetry migration 4
  adds explicit descending `seq` to both recent indexes, matching the real timestamp/sequence order.
  The canonical Observe suite now rejects the full-history outcome scan and either temporary sort.
  The fourth paired run confirms both access paths without temporary sorting. Exact outcome,
  provider, session, task and run plans remain separately reported; no unmeasured improvement is claimed.

## Writer ownership inventory

| Source | Finding and disposition |
| --- | --- |
| Daemon observer | `observer_factory.go` injects the daemon registry; it does not open a second global writer. |
| Independent handles | Admission, reentry detection and owner diagnostics identify the opened SQLite file with `os.SameFile`, including guarded session connectors. Separate handles for the same file share the queue; unrelated files remain independent. |
| Generated global mutations | The inventory found 104 direct repository mutations in 49 files. They now use named write admission. The five extension transfer operations already belonged to a caller-owned transaction and remain there. |
| Raw global mutations | Marketplace replacement, attention snapshots/mutes, automation deletion, dead-entity state and user-scope reconciliation now use the same transaction owner. Snapshot hashing/JSON and migration schema discovery happen before acquiring the writer. |
| Background producers | Catalog/process observations, health, token/permission observations, worktree status and bounded admission-claim sweeps use background execution. Authoritative lifecycle/authorization changes retain interactive admission. |
| WAL maintenance | Core and telemetry each own one maintenance connection, running only PASSIVE checkpoints outside writer admission. Runtime writers disable automatic checkpoints. Maintenance cannot reserve the writer/read pools or acquire the WAL writer lock; close cancels and joins it before the final checkpoint. |
| Composition-root adapters | Extension registry and config-apply mutations use named admission. Extension, settings, resources and observer task reads receive the independent read pool. |
| Extension CLI | Calls the daemon while available. The offline fallback acquires the same daemon file lock before opening the store and retains it through close; a running or concurrently starting daemon excludes it. |
| Provider-auth metadata | Uses a schema-validated read-only vault reader. It cannot migrate, checkpoint, initialize telemetry or recover state; a missing database remains absent. |
| Vault-key migration | Reads ciphertext through the same read-only reader; keyring writes retain the existing vault migration lock. |
| Demo seed | Its production entry point acquires the daemon lock before opening the global store. |

The queue does not synchronize arbitrary external SQLite clients. Supported production writers are
routed through the daemon or excluded by its process lock; in-process handles share file identity.
Startup migration/schema initialization owns the file before exposing repositories. Raw SQLite
locks remain represented separately from cancellation while waiting for admission in diagnostics.
The load experiment still must establish maximum holds for large atomic catalog/process operations.

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
| `globaldb.TestGlobalDBTelemetryRelocation` after review | Passed (0.550 s): after the first 128 source rows are acknowledged, a missing or empty destination is rejected; restoring it resumes without losing any of 129 events. |
| `globaldb.TestUserScopeMigration` after review | Passed (0.787 s): an actual version-150 home-workspace fixture upgrades and reopens with its relocated event and cursor intact; the complete workspace-column inventory includes the new outbox. |
| `globaldb.TestGlobalDBTelemetryDelivery` | Passed (0.735 s): full core writer occupation, second read connection, read-only enforcement, workspace tombstone/cleanup, durable outbox failure/replay. |
| `store.TestBackgroundWrites` | Passed (0.102 s) after queue deadline classification and per-producer attribution. |
| `daemon.TestDaemonToolEventSink` | Passed (0.712 s): a completed tool event survives a real telemetry writer lock and a canceled persistence attempt. The first run exposed a test seed copying only the core file; the shared seed now copies the full database family. |
| `store.TestProductionMigrationStreamsFreshReopenAndAhead/Should_fresh-apply,_reopen,_and_reject_an_ahead_telemetry_stream` | Passed (0.079 s) after the final index migration. |
| `profile.TestManagerProfileLifecycle/Should_preview_and_sweep_the_complete_delete_catalog` | Passed (0.287 s): preview and deletion retain the existing full ownership assertions. |
| `profile.TestManagerRecoveryAndReservation/Should_retry_telemetry_removal_before_completing_profile_deletion` | Passed (0.242 s): an actual SQLite delete failure leaves a hidden profile and retryable lifecycle; retry removes the retained telemetry before completion. |
| `daemon.TestBootInitializesRuntimeAssemblerIntegration` | Passed (1.013 s): real isolated daemon boot and shutdown; both persisted migration streams are reported. |
| `globaldb.TestGlobalDBHeartbeatWakeAuditStore` | Passed (0.810 s): durable event/state decision behavior remains intact. |
| `globaldb.TestGlobalDBWorktreeStore/Should_commit_an_exit_terminal_state_and_replay_event_atomically` | Passed (0.541 s) before main integration; CI reruns the integrated suite. |
| `globaldb.TestPageSessionsStableKeyset` after CI repair | Passed (0.928 s): the initialized read pool uses the registered SQLite connector, preserving Unicode search on every physical connection. |
| `sessiondb.TestSessionDBInternalWriteHelpers` after CI repair | Passed (0.047 s): the manually assembled drain fixture carries the production read connection; accepted-write assertions are unchanged. |
| `core.TestAuthoredContextProfileScope` after CI repair | Passed (0.755 s): the canonical seed clones the complete database family for each profile-isolation case. Browser runtime seeding now copies the same required sibling; its actual walk remains CI-owned. |
| `acp.TestPromptErrorPreservesRequestErrorData` after main integration | Passed (0.155 s): the rate-limit I/O fixture uses an internal-error response instead of the reserved authentication-required code. Classification/recovery assertions and production authentication semantics are unchanged. |

Additional ownership checks passed individually: `TestExecuteWrite` (0.218 s),
`TestBackgroundWrites` (0.079 s), `TestAttentionAcknowledgements` (0.472 s),
`TestGlobalDBVaultSecretsCRUD` (0.283 s), `TestAppMetadataFreshDB` (0.209 s),
`TestOpenSessionDBAppliesBaselineAndRepeatedBootIsIdempotent` (0.140 s),
`TestExtensionInstallOfflinePersistsExtension` (0.256 s), `TestProviderAuthStatusCommand` (0.111 s),
`TestVaultCommands` (0.269 s), and the tagged `TestRegistryIntegrationLifecycle` integration
function (0.105 s). The close-error repository fixture still preserves both independent errors
(`TestConfigApplyRecordRepositoryListApplyRecordsRowsCloseErrors`, 0.029 s). These cover actual
SQLite writer occupation, cross-handle priority/reentry/attribution, committed read isolation,
read-only vault access at schema 185, and offline exclusion by the daemon lock. Focused store and
globaldb lint reported zero issues. Settings and extension focused lint also passed after waiting for the shared runner lock.
The first attempts rejected by that lock are not passing evidence.

Review/CI remediation checks passed individually: `TestGlobalDBTelemetryRelocation` (8.022 s),
`TestGlobalDBTelemetryDelivery` (5.114 s), the atomic worktree terminal-event subtest (3.901 s),
`TestGlobalDBAutomationSuggestionsEnforceConsentInvariants` (4.602 s),
`TestGlobalDBSessionPromptAdmissionMigration` (20.057 s),
`TestDaemonManagedDeliveryRecovery` (14.060 s), stale transcript preparation subtests (0.132 s),
and `TestProductionMigrationStreamsFreshReopenAndAhead` (5.542 s). The migration fixtures now
use the production family owner and copy both SQLite files; their preservation assertions remain.
The existing auth-wait cancellation subtest passed (3.750 s) after canceling the daemon-owned wait
before the stop ladder can kill/rebind an idle runtime. No assertion was weakened.

The owning Atlas and sqlc generators completed after main integration. Full `make codegen` passed
before integration; `make codegen-check` was externally terminated and is not a passing receipt.
The test-convention heuristic reported existing legacy test shapes and file-wide parallel/environment
false positives; the new recovery and telemetry cases use the canonical subtest suites and check cleanup
errors. Formatting was restricted to the changed Go files. No local full/race/E2E suite or repository-wide lint/gate was run. Final formatting, lint, codegen,
build, unit and E2E delivery evidence belongs to CI on the PR head.

## Required work still in progress

The following acceptance items remain open and are not implied by the focused tests:

1. Same-machine before/after load with approximately 30 streaming sessions: p99 transcript/catalog
   latency, transactions/sec, core bytes/minute, zero interactive failures and no holds over one second.
2. Keep the isolated real-daemon walk green on the final head. Its first complete run passed on
   `c6037e2a3`, as detailed below; no operator credentials or runtime home were reused.
3. Measure the remaining large atomic catalog/process operations under the representative load;
   routing coverage is implemented but does not by itself prove the latency target.
4. Final-head CI plus two CodeRabbit reviews, followed by merge-train handoff; this agent does not merge.

### Complete ownership and review checkpoint

Focused lint across the 12 affected Go packages passed with zero issues (bounded concurrency 2,
serialized against other lint runners). The catalog store suite passed in 10.090 s, including
preparation with the sole writer connection occupied and invalidation by another database handle.
No performance conclusion is inferred from those correctness checks.

The batch catalog regression also passed (4.315 s): an earlier replacement that prunes a sibling
cannot reuse that sibling's prepared comparison. Scoped globaldb lint passed with zero issues.

Main integration receipts: relocation suite 7.555 s; all production migration streams 5.387 s;
main's Always-on upgrade suite 8.306 s; focused globaldb/ACP lint zero issues. Owning Atlas/sqlc
generation completed. All main migration bytes and checksum entries through version 186 match
origin/main. The relocation tail is now 187–190, with export at 188 and completion at 189.

### Dedicated acceptance lanes

`scripts/sqlite-contention/` and the `SQLite contention acceptance` workflow own the paired
baseline/head experiment. The identical Go workload runs against both commits on one Linux
runner, with sequential alternating trials. It records actual successful write-syscall bytes
separately from untraced latency; all-column upgrade hashes, Observe results, retired cursors,
index page counts/plans and startup times accompany the report. The failed experiments and their
numerical results are recorded below. Harness compilation, focused lint and a bounded trace-parser
check passed locally; no representative load ran on the shared host.

The separate daemon lane uses the existing runtime harness and an `eng-qa-bootstrap` targeted
manifest for API, CLI and runtime surfaces. The opt-in `contentionload` suite exercises thirty
scripted ACP streams, concurrent transcript/catalog reads and renames, then clean restart with
retained transcript and health reads. It records full daemon logs, strict audit and manifest-owned
teardown. This is a real daemon/storage/transport exercise; the ACP peer is scripted, with no
live-provider latency, authentication or full browser UX claim.

The tagged daemon acceptance suite compiles and its focused lint reports zero issues. Python
syntax, test-shape checks and workflow formatting pass. This validates the runner setup only;
the daemon load itself has not run locally. Commits bypass the repository-wide pre-commit hook
under the operator's CI-only gate rule, after the named focused checks.

| Acceptance walk | Current status | Evidence owner |
| --- | --- | --- |
| Paired 30-session store workload and 1 GiB upgrade | Upgrade passes; load acceptance still fails | Dedicated CI report and raw artifacts |
| Real daemon HTTP/SSE/UDS/CLI load and clean restart | Passes on `7b941a4ed`; final-head rerun required | `TestSQLiteContentionDaemonLoad`, bootstrap audit and teardown |

### First paired experiment: failed acceptance, retained evidence

[CI run 41747249696](https://github.com/compozy/compozy/actions/runs/41747249696) compared
baseline `d35c63c9a9` with `1f2f7cc94`. The 1,073,917,952-byte fixture contained 830,020 retained
summaries. Every column, sequence, retired cursor and Observe result survived the production
upgrade. The all-column SHA-256 was `e127f4cd408ae5bf01b7119d4d98108543a517cdc2c73547301befaeb742f944`.
Open/upgrade/reopen took 44.0 / 283,199.1 / 53.4 ms. These are Linux runner observations.

| Initial measurement | Baseline | Candidate |
| --- | ---: | ---: |
| Transcript p99, median of three trials | 59.761 ms | 36.001 ms |
| Catalog p99, median of three trials | 17.688 ms | 20.605 ms |
| Core write commit hooks/s | 238.32 | 62.35 |
| Core file-write bytes/min, traced | 79,049,881 | 109,967,957 |
| Operation failures, including warmup | 281 | 48 |

**FAIL:** candidate admission failures occurred during warmup. Copied fixture files were not
fsynced before process launch; the trace recorded expensive initial core/auxiliary/session syncs.
Fixture-copy writeback is a suspected contributor, not a proven sole cause. The corrected runner fsyncs
every copied file before launch. Warmup errors and long holds still fail acceptance. No numerical
improvement is accepted from this failed run, and the increased core write figure is retained.
Traced runs also had errors and do not establish equal completed throughput. The corrected tracer
uses seccomp filtering to avoid tracing unrelated syscalls. Session commit hooks are not available
through the separate guarded driver; their previous zero is not a measured count and is removed.

The next experiment also explicitly sweeps the first six hours of retained summaries while streams
run. Exact eligible/deleted counts prove that the large retention path was exercised, with the
same two-second interactive deadlines and a separate one-minute whole-sweep deadline.

### Fresh initialization and scoped publication repair

The first integrated CI run exposed a production startup regression: fresh global databases
replayed all historical migrations rather than applying the current declarative schema. Several
race jobs exhausted the unchanged 45-second initialization deadline. Fresh bootstrap now commits
the current schema, family identity and a pending initialization marker together, then binds the
telemetry sibling before exposing repositories. Reopening at either cross-file boundary resumes
initialization; a foreign sibling is rejected. Existing installations retain the ordered export
path and missing-sibling refusal after initialization completes.

CodeRabbit also identified a scope leak in publication waiting. Observe and watch readers now
wait only for foreground attempts in the profile/workspace their query can see. The existing
telemetry suite verifies that unrelated reads proceed and matching waits honor cancellation.

Focused receipts after these repairs: migration streams 10.909 s; telemetry relocation 1.015 s;
telemetry delivery 1.773 s; physical session-owner guards 0.146 s; heartbeat snapshot/revision
0.804 s; suggestion consent invariants 0.512 s; daemon managed-delivery recovery 7.755 s;
config MCP resource round trip 0.081 s; bundled Tailscale install 0.105 s; failed MCP probe stderr
0.473 s. Focused store/globaldb/sessiondb/harness lint reports zero issues.

The browser traces showed that changing from On-demand to Always-on cleared the selected
provider/model and produced HTTP 400. While this branch was being repaired, main shipped a
production fix that preserves an explicit runtime when the destination scope supports its provider.
Integration retains that fix and main's stronger browser assertions: selection before the kind
change, global scope plus provider/model in the POST, and a successful response before navigation.
No local reordering workaround remains. Browser execution remains CI-owned.

Integration also exposed the lint failures already present on main's `eb865c4f2` CI run. Repairs
preserve the existing assertions and behavior: explicit optional-interface checks, a stable
extension reference before the nested profile loop, consumption of the normalized derive error,
the equivalent `CutLast` cache-key split, and formatter/modernizer fixes in existing suites.
The focused extension projection case passed (0.198 s), parked-catalog preflight case (0.179 s),
HTTP committed-child cancellation case (0.045 s), discovery failure classification (0.018 s),
and workspace skill rescan (0.015 s). No broad local gate was run.

### Daemon acceptance fixture repair

[Daemon job 126322979263](https://github.com/compozy/compozy/actions/runs/41753188676/job/126322979263)
started the fresh global store in 16 ms and created all thirty sessions, but every prompt returned
HTTP 422 because the scripted ACP peer named a model without advertising its required model
configuration option. This is a failed acceptance run; it reached neither the load window nor the
restart assertions. The strict auditor correctly reported missing API/CLI evidence, and teardown
reported `clean: true`.

The fixture now advertises the same model through the ACP configuration contract. The owning
runtime test also reports an early prompt failure immediately and joins prompt goroutines on
failure. Its invariant remains thirty overlapping streams, successful interactive requests and
preserved accepted content after clean restart. Compilation passed (0.040 s) and tagged focused
lint passed with zero issues; actual execution remains CI-owned.

The tagged taint analyzer traced trusted bootstrap-manifest paths and loopback URLs into five
shared E2E helper files. The configuration exception is restricted to those exact test helpers and
G703/G704, matching the existing test-file boundary. It excludes no production path or other rule.
The experiment also rejects long-hold diagnostics in the traced head trial, in addition to the
untraced trials; tracing overhead is not an excuse for accepting an observed long hold.

### Declarative bootstrap schema parity

[Race shard 3](https://github.com/compozy/compozy/actions/runs/41753188407/job/126323432940)
found that fresh declarative bootstrap placed `automation_jobs.grant_ceiling` first, whereas the
published migration appends it last. Checking the existing owning assertions also found ordering
drift in `automation_scheduler_state`, `tasks` and `sessions`. Their declarative definitions now
match the existing upgraded schema, with sqlc regenerated from the matching static query columns.
The canonical schema/index assertions are unchanged; published migrations remain immutable.

### Second paired experiment: zero untraced errors, remaining long commits

[Paired job 126322979504](https://github.com/compozy/compozy/actions/runs/41753188676/job/126322979504)
compared the same baseline with `d36eb473f`. The 1 GiB upgrade again preserved all 830,020 rows,
every column and the same hash/cursors/Observe results. Open/upgrade/reopen took
39.0 / 280,875.4 / 50.2 ms. Each untraced trial swept all 43,169 eligible summaries while streaming.

| Second measurement | Baseline | Candidate |
| --- | ---: | ---: |
| Transcript p99, median of three untraced trials | 2,085.904 ms | 165.846 ms |
| Catalog p99, median of three untraced trials | 99.064 ms | 133.020 ms |
| Core write commit hooks/s | 155.83 | 58.41 |
| Core file-write bytes/min, traced | 484,845,593 | 43,301,696 |
| Operation failures, untraced including warmup | 708 | 0 |
| Long-hold/contention warnings, untraced | 570 | 2 |

**FAIL:** the candidate recorded a 1,381 ms coalesced transaction and a 1,000 ms interactive
state transaction in the second untraced trial. Neither generated a request error, but both
violate the lock-duration criterion. The traced candidate also had 133 operation failures,
four long transactions and an incomplete retention sweep (39,424/43,169 rows at its one-minute
deadline); its write rate does not establish equal completed throughput. The stricter runner
now checks traced warnings explicitly as well. These failed receipts remain evidence for the
next repair, not a passing performance claim. Long-hold diagnostics now include a code-owned database kind, callback duration and commit
duration, without logging a path or request data, to distinguish the remaining cause.

Focused receipts for this checkpoint: unchanged automation schema/index assertions 0.275 s,
task schema/index assertions 0.274 s, session registration/schema assertions 0.256 s, global
migration fresh/reopen/ahead/equivalence 6.227 s, and write owner/cancellation semantics 0.165 s.
Owning Atlas/sqlc generation completed without a new migration or changed historical checksum;
focused store/globaldb lint passed with zero issues.

The integrated vocabulary browser case failed because its global heading selector matched both
the window chrome and editor content. Its existing invariant belongs to the Agents E2E suite:
setup rows, the editor and window title retain the same noun through the old public slug. The
selector is now scoped to the existing page-content boundary, and both the window title and
content heading are asserted for Heartbeat and Instructions. No count/visibility assertion is
relaxed; browser execution remains CI-owned. The runtime E2E lane and Web shards 2/4 and 4/4
passed on `d36eb473f`; this is historical, not final-head acceptance.

### Real-daemon acceptance: passed on c6037e2a3

[Daemon job 126332046503](https://github.com/compozy/compozy/actions/runs/41755973452/job/126332046503)
completed all thirty overlapping ACP streams through the public daemon transports. All thirty
transcripts and the accepted health observations survived a clean restart. The full daemon logs
contained none of the targeted contention/long-hold failures. The strict API/CLI/runtime evidence
audit passed with zero blockers and warnings, and manifest teardown reported `clean: true` with
no survivors. Artifact `13288347365` retains the receipts.

| Public operation during streaming | Successful requests | p99 | Maximum |
| --- | ---: | ---: | ---: |
| Transcript | 3,000 | 110.885 ms | 171.194 ms |
| Session catalog | 199 | 94.757 ms | 110.965 ms |
| Session rename | 40 | 116.965 ms | 257.857 ms |

This verifies the real daemon/storage/transport path with a scripted ACP peer, not live-provider
authentication or browser rendering. The separate large-retained-data experiment still owns the
before/after performance criterion and remains open.

The measured index-plan repair passed the canonical recent Observe suite (0.274 s), telemetry
fresh/reopen/ahead/schema-equivalence suite (0.034 s), and focused globaldb lint (zero issues).
The owning generators appended telemetry migration 4; every previous migration/checksum remains
unchanged. Its large-fixture page counts and query plans will be re-measured in CI.

### Third paired experiment: indexed retention and commit attribution

[Paired job 126332046840](https://github.com/compozy/compozy/actions/runs/41755973452/job/126332046840)
tested `c6037e2a3`. All three untraced candidate trials had **zero operation errors and zero
long-hold/contention warnings**, and removed all 43,169 eligible rows. The baseline had 42 errors
and 32 warnings. Transcript/catalog p99 medians were 57.112/23.174 ms versus 46.601/18.943 ms;
core commit hooks/s were 63.16 versus 227.65. Upgrade/reopen took 282,318.5/57.4 ms with the same
830,020 rows, all-column hash, sequence high-water, integrity and Observe parity.

**Acceptance still FAIL:** the traced candidate had 230 operation failures and removed only
35,328/43,169 eligible rows. All four long transactions belonged to telemetry retention. Their
callback/commit durations were 1,428/975, 1,763/2,191, 1,496/1,487 and 1,376/192 ms. The traced
core bytes/min were 481,482,961 before and 45,826,961 after, again without equal completed
throughput. The raw artifact is `13289155589`; this is not an accepted performance improvement.

The repair bounds indexed deletion to 64 rows per atomic item and moves automatic WAL checkpoint
work off core/telemetry COMMIT paths. Each store owns an independent one-connection maintenance
pool, opened with `mode=rw` so it cannot create a missing file. A one-second worker uses PASSIVE
checkpoints, leaving incomplete work for later ticks and preserving FULL telemetry commit sync.
It is canceled and joined before final checkpoint/close, including failed-open cleanup. The
existing embedded SQLite version is 3.54.1; no dependency change is involved. SQLite documents
the [PASSIVE nonblocking behavior and NOOP progress query](https://www.sqlite.org/pragma.html#pragma_wal_checkpoint).

The canonical checkpoint suite verifies that committed frames reach the database while a live
transaction reserves the sole writer connection, that maintenance closes without waiting for
that transaction, and that the transaction still commits afterward. Performance acceptance will
be rerun; no deadline, failure condition or durability assertion was relaxed.

Focused receipts: checkpoint lifecycle 1.050 s; global opening/read isolation 0.834 s; telemetry
delivery 1.782 s; retention cutoff 0.377 s; interrupted/family-bound relocation 0.851 s. Focused
store/globaldb/telemetrydb lint passed with zero issues. No broad local gate ran.

CodeRabbit's GitHub full review exceeded 300 files. Its existing CLI completed a deep review of
all 115 files in `1f2f7cc94..c6037e2a3`, extending the three completed GitHub reviews (280, 14 and
145 files). The valid in-scope finding about missing teardown receipts is repaired in `486a7fe2a`.
The Go 1.26 compatibility finding is contradicted by the current module and CI version, 1.27.3.
Seven observations concern files identical to current main and are disclosed in PR comment
`6620645196`; none is claimed as repaired by this issue. Greptile reviewed all 380 files at
`c6037e2a3` with zero new comments. Final-delta review remains required.

### Fourth paired experiment: remaining interactive stall

[Paired job 126342783269](https://github.com/compozy/compozy/actions/runs/41759267689/job/126342783269)
tested `7b941a4ed`. All three untraced candidate trials completed the 43,169-row sweep with zero
operation failures, versus 466 baseline failures. One candidate core `update session state`
transaction still held the lock for 1,026 ms (956 ms callback, 69 ms commit). **Acceptance FAIL.**
Transcript/catalog p99 medians were 141.673/92.187 ms versus 579.439/42.009 ms; core commit hooks/s
were 59.60 versus 171.52. The upgrade preserved the same all-column hash, 830,020 rows, cursors,
integrity and Observe results; open/upgrade/reopen took 34.4/310,043.5/48.2 ms.

The traced candidate had 50 operation failures, removed 31,296/43,169 rows before the one-minute
sweep deadline, and logged one 1,143 ms telemetry transaction (131 ms callback, 1,011 ms commit).
Traced core bytes/min were 53,573,276 versus 488,266,218, without equal completed throughput.
Artifact `13290880455` preserves this failed experiment. The checkpoint/64-row repair removed the
previous multi-second retention callbacks; it has not yet satisfied the complete load criterion.

The remaining untraced core callback consists of indexed workspace/session checks and a single
session update; attribution alone does not identify whether the delay is SQLite, I/O or runtime
contention. A separate diagnostic replay now records Go execution tracing after a failed acceptance
run. Named workload regions distinguish scheduling, GC, Go locks and syscalls. Diagnostic timing
does not replace any acceptance sample, and the original failure remains a failing job.

The corrected recent indexes are now measured on the large fixture: profile-recent reads search
`idx_summaries_profile_timestamp`, and error-only reads scan only the partial
`idx_summaries_failures_timestamp`, both without a temporary sort. The partial index occupies
4,096,000 bytes versus the former full outcome index's 44,470,272 bytes. Exact-outcome reads still
scan the timestamp index, and provider/session/task/run sorts remain explicitly reported; this
change makes no latency claim for those paths.

[Daemon job 126342783474](https://github.com/compozy/compozy/actions/runs/41759267689/job/126342783474)
passed again: 30 completed streams and preserved transcripts, strict audit with zero blockers or
warnings, and clean teardown without survivors. Transcript/catalog/rename p99 were
100.671/119.854/153.479 ms across 3,000/198/40 successful requests. The complete log scan also
found zero passive-checkpoint errors; artifact `13289657954` contains the receipts.

CodeRabbit settled the seven main-identical observations as outside this PR and withdrew the Go
version false positive in comment `6620660789`. Its CLI then reviewed all 13 files in
`c6037e2a3..7b941a4ed`, finding only that a teardown `clean` value must be a boolean. Commit
`89a3dff28` fixes that and rejects checkpoint errors in both load verifiers. Six focused receipt
failure cases passed; the three-file follow-up CodeRabbit review completed with zero findings.
Tagged harness compilation and focused lint passed with zero issues. Final-head CI/review remains
required after the unresolved performance cause is repaired.

The next main integration preserves main's shutdown/revocation repairs and the branch's
daemon-owned authentication-wait cancellation. Conflicting lint-only changes take main's exact
form. Main now renders editor content as `h2`; the owning Agents browser suite keeps its unique
global `h1` assertion and also verifies the scoped `h2` content noun. No behavior assertion is
removed. No schema/dependency change arrived with this merge.

The prior-head Web shard 3 failed the custom-skill removal walk. Its trace/log records navigation
while the settings PATCH was still applying, followed by HTTP 499 and canceled publication; the
test had waited only for desired config to appear in GET. The settings contract separately reports
`applied` after runtime reconciliation. The existing skill-sources E2E now awaits that same PATCH
and asserts HTTP 200 plus `applied: true` before navigation. The persisted-config and absent-picker
assertions and their deadlines remain unchanged. The rerun remains CI-owned.

Focused integration receipts: discovery failure classification 0.016 s, canceled authentication
wait 0.259 s, daemon-shutdown exit classification 1.092 s, and focused session/modelcatalog lint
with zero issues. Both changed browser files pass formatting; their actual walks run only in CI.

### Transaction-abort isolation and current daemon receipts

The fourth experiment exposed a cancellation that destroyed the enclosing SQLite transaction:
the item rollback reported `no such savepoint`, followed by `no transaction is active`. The batch
writer previously returned that failure to healthy peers too. A deterministic real-SQLite
`INSERT OR ROLLBACK` case in the canonical `TestBackgroundWrites` reproduced the defect: both
healthy peers failed and zero rows persisted. The repair retains failed-item outcomes and replays
only healthy/unexecuted peers separately after rollback/discard. Individual contexts, priority,
authorization fences and durable acknowledgements use the existing write helper. Commit failures
never enter this fallback. Callbacks retain the replayability already required for BUSY retries.

Focused receipts: new red/green regression 0.029 s, complete `TestBackgroundWrites` function
0.079 s, complete `TestExecuteWrite` function 0.265 s, test conventions and scoped store lint with
zero issues. The diagnostic runner also preserves the original acceptance failure when its extra
trace fails, writing a separate error receipt and removing its copied fixture. Four focused
success/exit-error/timeout/invalid-JSON cases verified that boundary.

[Daemon job 126353809248](https://github.com/compozy/compozy/actions/runs/41762695929/job/126353809248)
passed on `bc1c46432`: 30 completed streams and preserved transcripts, zero strict-audit blockers
or warnings, clean teardown and no survivors. Transcript/catalog/rename p99 were
126.319/83.395/246.742 ms across 3,000/196/40 successful requests. All six log files contained zero
long-hold, retry-exhaustion, passive-checkpoint or completed/rolled-back transaction errors.
Artifact `13290493945` preserves this run. Paired-load acceptance and final-head CI remain open.
