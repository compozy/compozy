## TC-INT-002: Timeout-Class Routing and Public Run-Reader Compatibility

**Priority:** P1 (High)
**Type:** Integration
**Status:** Passed
**Estimated Time:** 22 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing with E2E follow-up
**Automation Command/Spec:**
- `go test ./internal/api/client -run 'Test(ClientUsesCanonicalTimeoutClassesByRoute|ClientRemoteErrorsDecodeCanonicalEnvelopeAndRequestID|GetRunSnapshotPreservesCanonicalFields|OpenRunStreamReconnectsFromLastAcknowledgedCursorAfterHeartbeatGap)' -count=1`
- `go test ./pkg/compozy/runs -run 'Test(OpenAndListUseDaemonBackedHTTPTransport|TailAndWatchWorkspaceUseDaemonBackedHTTPTransport|OpenMatchesInternalClientSnapshotMetadata|RemoteWatchAndClientStreamSurviveHeartbeatIdlePeriod|OpenLoadsDaemonBackedRunSummary|OpenSurfacesStableDaemonUnavailableError|AdaptRemoteRunSnapshotPreservesIncompleteReasons|WatchRemoteReconnectsAfterOverflowWithoutDuplicatingEvents|WatchRemoteStopsAfterHeartbeatEOFWhenSnapshotIsTerminal|ReplayPagesEventsInOrder|ReplayReportsIncompatibleSchemaVersion)' -count=1`
**Automation Notes:** Existing automation proves timeout routing, remote error decoding, snapshot compatibility, reconnect behavior, and run-reader semantics. Task `09` should still exercise at least one live-daemon run-reader path because current package coverage uses in-process/test-server fixtures.

### Objective

Verify that daemon-facing clients honor the TechSpec timeout policy and that `pkg/compozy/runs` continues to preserve public open/list/tail/watch/replay behavior against canonical daemon payloads.

### Preconditions

- [ ] The daemon client and `pkg/compozy/runs` packages remain on the canonical contract boundary.
- [ ] The execution branch still exposes the public run-reader APIs without hidden compatibility shims outside the package boundary.

### Test Steps

1. Run the focused daemon-client suite listed above.
   **Expected:** Timeout classes, canonical remote-error decoding, snapshot field preservation, and heartbeat-gap reconnect behavior all pass.

2. Run the focused `pkg/compozy/runs` suite listed above.
   **Expected:** Open/list/tail/watch/replay behavior remains compatible and stable for the public reader surface.

3. Confirm sticky `Incomplete` reasons and replay semantics remain preserved.
   **Expected:** Run snapshots and replay pages expose integrity state without silently dropping reason codes or duplicating events.

4. Record the remaining live-daemon follow-up needed for `task_09`.
   **Expected:** A real daemon-backed open/list/watch proof remains explicitly planned, not assumed from package fixtures.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Probe vs long mutate | Health probe vs task start route | Different timeout classes are selected correctly |
| Remote conflict/schema error | Canonical error envelope from daemon | Request ID and error semantics are preserved |
| Heartbeat gap | No event arrives within gap tolerance | Stream reconnects from last acknowledged cursor |
| Sticky incomplete reason | Snapshot with recorded integrity issue | Public reader preserves the reason codes |
| Replay after schema mismatch | Incompatible replay payload | Stable compatibility error instead of silent corruption |

### Related Test Cases

- `TC-INT-001`
- `TC-INT-005`
- `TC-FUNC-002`

### Traceability

- TechSpec `Timeout Policy`
- TechSpec `Snapshot Integrity Semantics`
- ADR-001: canonical daemon transport contract
- ADR-004: observability as a first-class contract
- Task reference: `task_04.md`

### Notes

- This is the main compatibility guardrail for daemon clients and public run readers. Treat it as a distinct regression bucket instead of assuming CLI coverage is sufficient.
