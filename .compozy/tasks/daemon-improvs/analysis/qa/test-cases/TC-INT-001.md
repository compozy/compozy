## TC-INT-001: Canonical HTTP/UDS Parity and SSE Resume

**Priority:** P0 (Critical)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 20 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/api/httpapi -run 'Test(HTTPAndUDSServeMatchingStatusSnapshotAndConflict|HTTPAndUDSServeCanonicalParityAcrossRouteGroups|HTTPAndUDSEmitEquivalentCanonicalSSEStreams|HTTPStreamResumesAfterLastEventIDAndEmitsHeartbeat|HTTPStreamRejectsInvalidAndStaleCursor|MetricsAndTerminalStreamRemainObservable)' -count=1`
- Supporting seam: `go test -tags integration ./internal/api/contract -run 'Test(DaemonHealthRouteDecodesIntoCanonicalContract|RunSnapshotAndStreamDecodeIntoCanonicalContract)' -count=1`
**Automation Notes:** Current coverage proves parity at the transport/server layer. Task `09` must still seek live-daemon dual-transport proof because these are not full operator E2E tests yet.

### Objective

Verify that the canonical contract remains symmetric across HTTP and UDS: status codes, envelope shapes, request IDs, SSE events, heartbeats, overflow semantics, and stream-resume behavior must match for the shared daemon route inventory.

### Preconditions

- [ ] The transport integration suite can bind loopback HTTP and create UDS sockets in the test environment.
- [ ] The branch still uses `internal/api/contract` as the canonical transport boundary.

### Test Steps

1. Run the focused `internal/api/httpapi` parity suite listed above.
   **Expected:** The package exits `0` and all listed parity/SSE subtests pass.

2. Confirm the suite validates both regular response parity and run-stream parity.
   **Expected:** HTTP and UDS return equivalent status/snapshot/conflict behavior and equivalent canonical stream items.

3. Run the tagged contract decode seam.
   **Expected:** Health, snapshot, and stream payloads decode into the canonical contract types without drift.

4. Record any transport-only pass that still lacks a managed-daemon proof.
   **Expected:** `task_09` gets an explicit live-daemon E2E follow-up item rather than silently inheriting false confidence.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Invalid or stale cursor | Resume with bad cursor state | Stable validation or stale-cursor response |
| Heartbeat-only idle period | Stream idle without new events | Heartbeat frames remain observable and reconnect-safe |
| Conflict path | Canonical conflict-producing route | Same status code, envelope, and request ID across transports |
| Terminal stream observability | Metrics plus terminal stream behavior | Observable transport state without contract drift |

### Related Test Cases

- `TC-INT-002`
- `TC-INT-005`

### Traceability

- TechSpec `API Endpoints`
- TechSpec `Testing Approach`
- ADR-001: canonical daemon transport contract
- ADR-003: validation-first daemon hardening
- Task reference: `task_03.md`

### Notes

- This is the main parity guardrail for the feature. It is `Integration`, not `E2E`, because the current harness does not boot a managed daemon for both transports.
