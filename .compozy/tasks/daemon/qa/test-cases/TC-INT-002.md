## TC-INT-002: UDS and HTTP Transport Parity

**Priority:** P0 (Critical)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/api/httpapi -run 'Test(HTTPAndUDSRegisterMatchingRoutes|HTTPServerPersistsActualPortInDaemonInfo|UDSServerCreates0600Socket|HealthTransitionsOverHTTP|HTTPAndUDSServeMatchingStatusSnapshotAndConflict|HTTPStreamResumesAfterLastEventIDAndEmitsHeartbeat|HTTPStreamRejectsInvalidAndStaleCursor)' -count=1`
- Supporting seam: `go test ./internal/api/core -run 'Test(StreamRunRejectsInvalidLastEventID|StreamRunEmitsHeartbeatAndOverflowFrames|StreamRunAdditionalBranches)' -count=1`
**Automation Notes:** This is the core transport-contract proof for ADR-003. It validates route parity, health/status behavior, SSE resume semantics, and stale/invalid cursor handling without inventing a second API harness.

### Objective

Verify that the daemon exposes equivalent behavior over UDS and localhost HTTP for the public transport surfaces required by CLI, TUI, and future local clients.

### Preconditions

- [ ] The environment supports Unix sockets and loopback HTTP on `127.0.0.1`.
- [ ] No external service occupies the ephemeral local port chosen by the tests.

### Test Steps

1. Run the focused transport-integration suite listed above.
   **Expected:** The package exits `0` and all parity/status/SSE tests pass.

2. Confirm the suite covers matching route registration, persisted HTTP port info, `0600` UDS permissions, health transitions, matching status/conflict behavior, and `Last-Event-ID` resume.
   **Expected:** Both transports expose the same operator contract and SSE remains resume-capable.

3. Run the supporting API-core handler suite when lower-level cursor/heartbeat evidence is needed.
   **Expected:** Invalid cursors are rejected consistently and heartbeat/overflow frames stay stable.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Invalid cursor | Bad `Last-Event-ID` | Stable validation failure |
| Stale cursor | Cursor older than retained window | Stable stale/overflow handling |
| Loopback port info | HTTP transport startup | Actual chosen port is persisted for clients |
| UDS permissions | Socket file created on boot | Socket remains private (`0600`) |

### Related Test Cases

- `TC-INT-001`
- `TC-FUNC-006`
- `TC-INT-003`

### Traceability

- TechSpec Unit Tests: UDS and HTTP status mapping parity, request-id/error envelope shape, SSE cursor behavior.
- TechSpec Integration Tests: UDS and localhost HTTP serve the same route behavior for status, workspaces, task runs, review runs, and run streams.
- ADR-003: AGH-aligned REST transports over UDS and localhost HTTP.

### Notes

- Any divergence here invalidates CLI/TUI/API parity claims, so this remains a P0 suite even when the functional smoke cases are green.
