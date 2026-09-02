# BUG-20260901-unselected-route-gate-executes: An unselected route gate still executes

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-partial-loop, settle an exclusive route and inspect its history
- **Scenarios:** LP-exclusive-route-history
- **Found:** 2026-09-01 · **Report:** docs/qa/reports/2026-09-01-loop-route-selection.md

## Summary

After a route selected one path, a later gate on the unselected path could still execute during the same planning pass. That gate could stop the selected work or publish a verdict for a path the Loop never took.

## Reproduction

- **Charter:** CH-loop-partial-runtime · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US; isolated runtime harness with real HTTP, UDS, CLI, and Loop runtime paths

1. Publish a Loop whose route selects `quality` and whose mandatory default points to the lexically later `z_publish_guard` gate.
2. Start the Loop through HTTP and wait for the selected `quality` path to settle.
3. Read the run through `compozy loop status -o json` over UDS.

**Expected:** Only `quality` has a verdict; `z_publish_guard` and its descendant remain `route_not_taken`.
**Actual:** Before the fix, `z_publish_guard` could execute from a stale planning snapshot after the route had already marked it unselected.

## Evidence

- `internal/loop/coordinator_control_test.go` — focused planner regression.
- `internal/daemon/loop_generation_feedback_e2e_integration_test.go` — HTTP start plus CLI/UDS read-back regression and adjacent runtime canaries.
- `docs/qa/reports/2026-09-01-loop-route-selection.md` — commands, results, and parity limits.

## Fix

- **Root cause:** the planner iterated copied generation outputs and did not reload the canonical output after route selection changed later nodes in the same pass.
- **Fix commit:** `304059507bbeff0213b1d516cccbd5be7939bb03`
- **Regression test:** `TestCoordinatorRunnerShouldNotEvaluateUnselectedRouteGate`; the daemon route-history case now exercises the same gate shape through HTTP and CLI/UDS.

## Verification

- **Retested:** 2026-09-02, same Bruno route-history journey · **Report:** docs/qa/reports/2026-09-01-loop-route-selection.md
- **Result:** The focused race regression passed, and the full daemon feedback integration suite completed in 46.781 seconds. HTTP start plus CLI/UDS read-back showed no `z_publish_guard` verdict, kept the gate and its descendant `route_not_taken`, and settled the selected path `done`.
