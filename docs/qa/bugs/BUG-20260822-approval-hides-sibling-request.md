# BUG-20260822-approval-hides-sibling-request: Approval prevents an independent request from opening

- **Status:** verified
- **Impact (user-side):** Blocks-Operation
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada, Loop run operator
- **Journey Step:** Resolve every item in Needs you
- **Scenarios:** LP-web-needs-you-gate-provenance, LP-web-pending-request-provenance
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

Control planning returned as soon as a human gate produced `needs-approval`. An independent ask
node in the same generation stayed pending, so its durable request never opened and the public
Needs you register reported only one of two blockers.

## Reproduction

- **Invariant:** A `needs-approval` gate parks the run without preventing other ready,
  dependency-independent control nodes from materializing their durable waits and requests.
- **Owning layer:** Loop coordinator control planning.
- **Canonical suite:** `TestCoordinatorRunnerShouldParkGateApprovalWait` in
  `internal/loop/coordinator_control_test.go`.

**Expected:** The run is `needs-approval`, while both the approval wait and independent ask request
are durable and visible.
**Actual:** The approval wait is durable, but the ask output remains pending and `loop_requests` is
empty.

## Fix

Control planning retains the first approval terminal while continuing to evaluate other ready
control nodes. Non-approval terminals still stop planning immediately.

## Verification

- Canonical gate and ask control tests: 5 passed.
- Real daemon-served Playwright E2E-013 journey: 1 passed in 52.0 seconds; two Needs you items were
  ordered and counted.

