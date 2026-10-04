# Issue 689: SQLite contention verification

## Scope and root cause

The orphan sweep acquired `BEGIN IMMEDIATE` before discovering candidates. Its correlated
subqueries repeatedly scanned historical task runs without a general Loop ownership index.
Provenance backfill reread all coordinator history under the writer on every periodic cycle.
Thus an empty report could still monopolize writes. The original real-SQLite regression failed
with `begin immediate sqlite write: context deadline exceeded` over 12,000 settled task runs.

Action completion used a single bounded write and converted persistence failures into action
failures. Event-summary persistence could similarly replace an already-returned tool result with
an observability error. A failed action heartbeat could cancel the running tool. Separately,
scheduled claims retried immediately because the unchanged due time was already in the past.

## Acceptance evidence

Canonical suites use isolated temporary SQLite files and production stores, task services,
coordinator settlement, and scheduler claims. Tools/session creation are simulated only at their
I/O boundary. No user database or other worktree is used.

| Contract | Owning evidence |
| --- | --- |
| Settled history does not need the writer | `TestGlobalDBLoopTerminalReconciliationShouldConvergeExecutionRecords`: 12,000 historical rows while another connection holds `BEGIN IMMEDIATE`; empty sweep and backfill succeed. |
| Completed and failed results survive repeated write deadlines | `TestLoopGoalManagedRuntimeIntegration/Should_retain_an_executed_action`: real writer admission held beyond two persistence attempt windows; one invocation, terminal task run, released lease and an explicitly `succeeded` generation output (or `failed` for the failed action). |
| Heartbeat contention does not cancel an executing action | `TestLoopGoalManagedRuntimeIntegration/Should_retain_action_ownership`: same real writer pressure during execution. |
| Pending results survive shutdown and lease deadline passage | `TestLoopGoalManagedRuntimeIntegration/Should_retain_a_completed_result`: real writer contention during shutdown grace and a clock advanced beyond the original lease. |
| Recovery respects pending settlement ownership | `TestTaskManagerApprovalGateAndAttemptExhaustionIntegration/Should_fence_pending_settlement`: real task service/store; unrelated expired runs recover while a reserved run remains fenced, and releasing the reservation restores recovery eligibility. |
| Multiple coordinator histories produce one provenance source | `TestGlobalDBLoopTerminalReconciliationShouldConvergeExecutionRecords/Should_select_the_latest`: latest queued coordinator wins, its Loop ID/name stay aligned, and the second backfill is a no-op. |
| Completed tool events persist once after unlock | `TestDaemonToolEventSink`: held real writer and repeated persistence attempt expiry, followed by one stored event. |
| Scheduler does not spin and retains fire identity | `TestSchedulerIntegrationSQLiteContention`: real SQLite claim failure, injected clock, one-second then two-second retry delays, one original fire after unlock. |
| Empty wake retention does not need the writer | `TestGlobalDBHeartbeatWakeAuditStore`: another writer remains locked while no-op retention succeeds. |
| Upgrade preserves task history | `TestOpenGlobalDBReopenPreservesRowsAndStatus`: version 125 task metadata and result survive migration 126 and repeated reopen. |
| Managed session/tool access under shared contention | Existing `TestHarnessContextIntegrationMeasuresDeliveredSkillCatalogs/Should_load_a_managed_skill_resource` passes with a real ACP subprocess and concurrent session-health writes. |
| Migration integrity and equivalence | Existing `internal/store` production stream, apply, integrity and schema-equivalence suites. |

The affected backend portions of `LP-terminal-loop-settlement` and
`TA-automation-crud-loop-target` are covered by these real-runtime integration walks. Historical
browser evidence and unrelated scenario gaps are unchanged.

## Delivery validation

- Focused contention and migration suites passed with `CGO_ENABLED=1` and `-race`; the managed ACP skill/session contention scenario also passed.
- The test-convention checker reports no new findings relative to the unchanged suite baseline.
- `make codegen` uses the owning Atlas/sqlc generators; migration 00126 only adds indexes.
- `make gate` passed all affected lanes: `make codegen-check`, Go lint with zero issues, and the
  automation, daemon, store and task race suites. Further delivery validation uses CI; no additional
  heavy local suites are required. Current-head CI and review disposition are
  recorded separately in the delivery PR.
