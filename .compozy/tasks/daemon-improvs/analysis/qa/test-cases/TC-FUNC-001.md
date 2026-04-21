## TC-FUNC-001: Daemon Control Plane Lifecycle, Stop Semantics, and Logging Policy

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'TestDaemonStatusAndStopCommandsOperateAgainstRealDaemon' -count=1`
- Supporting seam: `go test ./internal/daemon -run 'Test(ManagedDaemonStopEndpointShutsDownAndRemovesSocket|ManagedDaemonRunModesControlLogging)' -count=1`
**Automation Notes:** The public CLI suite proves operator-visible start/status/stop behavior against a real daemon instance. The daemon integration seam proves socket cleanup and foreground-vs-detached log policy.

### Objective

Verify that the daemon control plane remains usable and observable after runtime hardening: status reflects real state, stop requests drain or force-stop correctly, and logging mode stays consistent with foreground vs detached execution.

### Preconditions

- [ ] Real-daemon CLI integration tests can launch with isolated home/runtime directories.
- [ ] Loopback HTTP port allocation is available for the managed daemon tests.
- [ ] The branch still uses the daemon command surface from `internal/cli`.

### Test Steps

1. Run the focused CLI daemon suite listed above.
   **Expected:** The package exits `0` and `daemon status`, `daemon start`, and `daemon stop` all pass against a real daemon instance.

2. Confirm the CLI suite validates both JSON and text output for stopped and ready states.
   **Expected:** The operator-visible payload reflects daemon readiness, PID, and HTTP port consistently.

3. Run the supporting daemon integration seam.
   **Expected:** Managed daemon stop removes socket/runtime artifacts, and foreground vs detached runs honor the expected logging behavior.

4. Review failures alongside the runtime and logger tests if a root-cause investigation is needed.
   **Expected:** Boundaries between CLI orchestration, daemon lifecycle, and log sink policy stay clear.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Status while stopped | `compozy daemon status` before start | Reports `stopped` cleanly |
| Ready daemon | `compozy daemon start` then `status` | Reports ready state, healthy status, PID, and bound loopback port |
| Graceful stop | `compozy daemon stop` | Stop accepted and daemon transitions to stopped |
| Foreground vs detached logging | Managed daemon started in different modes | Foreground mirrors to stderr; detached writes file-only structured logs |

### Related Test Cases

- `TC-INT-003`
- `TC-INT-005`

### Traceability

- TechSpec `Monitoring and Observability`
- TechSpec `Technical Dependencies`
- ADR-002: runtime supervision hardening
- ADR-003: validation-first daemon hardening
- ADR-004: observability as a first-class contract
- Task reference: `task_05.md`

### Notes

- This is the public-facing sanity check for the runtime hardening work. If it fails, deeper daemon execution should stop until the control plane is trustworthy again.
