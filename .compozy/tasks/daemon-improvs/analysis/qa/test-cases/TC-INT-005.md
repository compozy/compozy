## TC-INT-005: Observability Contracts, Snapshot Integrity, and Transcript Replay

**Priority:** P0 (Critical)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 22 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing with P0 live-daemon follow-up
**Automation Command/Spec:**
- `go test ./internal/daemon -run 'Test(ServiceStatusHealthAndMetricsReflectRuntimeState|RunManagerSnapshotIncludesJobsTranscriptAndNextCursor|RunManagerOpenStreamReplaysAllPersistedPages|RunManagerExecRunCompletesAndReplaysPersistedStream|LoadRunIntegrityMergesNewReasonsIntoStickyState|AuditSnapshotIntegrityDetectsEventGapAndMissingTerminalEvent|AssembleSnapshotTranscriptBoundsMessagesAndBytes)' -count=1`
- `go test ./internal/api/httpapi -run 'Test(HealthTransitionsOverHTTP|HTTPAndUDSServeMatchingStatusSnapshotAndConflict|MetricsAndTerminalStreamRemainObservable)' -count=1`
- `go test -tags integration ./internal/api/contract -run 'Test(DaemonHealthRouteDecodesIntoCanonicalContract|RunSnapshotAndStreamDecodeIntoCanonicalContract)' -count=1`
- Supporting run-reader seam: `go test ./pkg/compozy/runs -run 'Test(AdaptRemoteRunSnapshotPreservesIncompleteReasons|ReplayPagesEventsInOrder|ReplayReportsIncompatibleSchemaVersion|TailReplaysHistoryThenFollowsStreamWithoutDuplicates)' -count=1`
**Automation Notes:** Existing automation proves the contract strongly at service, transport, and public-reader levels, but most of it is still not a managed-daemon E2E path. Task `09` must capture live-daemon evidence for health/metrics/snapshot behavior.

### Objective

Verify that the richer observability contract stays intact: health and metrics expose the promised runtime state, snapshot integrity reasons remain sticky and durable, and transcript replay continues to reconstruct deterministic cold-reader output.

### Preconditions

- [ ] The branch still persists `run_integrity` state and transcript projections.
- [ ] Tagged contract integration tests remain runnable with `-tags integration`.
- [ ] The run-reader package still consumes the stronger snapshot contract.

### Test Steps

1. Run the focused daemon observability command listed above.
   **Expected:** The package exits `0` and health, metrics, integrity, snapshot, and transcript tests all pass.

2. Run the focused transport and tagged contract decode commands.
   **Expected:** Operator-visible health, snapshot, and stream payloads remain aligned with the canonical contract.

3. Run the supporting run-reader replay/integrity seam.
   **Expected:** Public replay and tail behavior preserve incomplete reasons and deterministic event order.

4. Record the required live-daemon follow-up for `task_09`.
   **Expected:** A managed-daemon proof for health/metrics/snapshot output remains explicit in the regression suite.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Sticky integrity reasons | Multiple later audits on same run | Reasons merge durably rather than being overwritten |
| Event gap or missing terminal event | Persisted run history anomaly | Snapshot marks `Incomplete` with durable reason codes |
| Transcript bounds | Large transcript projection | Cold replay remains bounded and deterministic |
| Contract decoding | Health/snapshot payloads | Canonical contract types decode without drift |
| Replay after idle/history mix | Historical page plus live stream | No duplicate events and stable ordering |

### Related Test Cases

- `TC-INT-001`
- `TC-INT-002`

### Traceability

- TechSpec `Monitoring and Observability`
- TechSpec `Snapshot Integrity Semantics`
- ADR-004: observability as a first-class contract
- ADR-001: canonical daemon transport contract
- Task reference: `task_07.md`

### Notes

- Treat observability as a primary daemon contract, not a reporting afterthought. Any drift here is operator-visible and should be handled as a real regression.
