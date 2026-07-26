# BUG-20260712-goal-judge-fixture-model: Goal runtime E2E cannot start its configured judge model

- **Status:** fixed
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea and Bruno as release operators relying on the Goal gate
- **Journey Step:** J-26 Goal lifecycle precondition
- **Scenarios:** GL-004; GL-005; GL-006 (automated fixture evidence; user verdicts unchanged)
- **Found:** 2026-07-12 · **Report:** docs/qa/reports/2026-07-12-hermes-bridge.md
- **Origin:** n/a

## Summary

The release operator cannot obtain a green runtime E2E gate because the deterministic Goal judge is configured as `goal-e2e-judge` while the acpmock agents do not advertise that model as an ACP session option. The daemon truthfully rejects the unavailable configuration, so five Goal lifecycle cases fail before testing their promised behavior.

## Reproduction

- **Charter:** Task 10 automated precondition · **Tour:** runtime contract gate
- **Environment:** local Linux, race-enabled `make test-e2e-runtime`, build `62ab3bc`

1. Run `make test-e2e-runtime` from the repository root.
2. Observe `TestDaemonE2EGoalCommandsShouldSurviveControlsDisconnectAndRestart`.
3. Inspect the Goal snapshot and session events after its first judge attempt.

**Expected:** The fixture advertises the configured judge model, the mock judge runs, and rejection/pause/approval/restart behavior reaches its contract assertions.
**Actual:** The snapshot enters `needs-approval` with `goal_judge_broken`; the provider error says the preferred model requires an ACP model config option.

## Evidence

- Initial full lane output retained in this Task 10 execution transcript: 59 tests, five Goal failures, `goal_judge_broken`.
- Canonical owner: `internal/daemon/loop_goal_command_e2e_integration_test.go` and `internal/testutil/acpmock/testdata/goal_command_fixture.json`.
- Introduction order: strict model negotiation landed in `dadf35f1f`; the Goal fixture was added later in `499cc7a5d9` without the required advertised option.

## Fix

- **Root cause:** The deterministic fixture declared `judge = "goal-e2e-judge"` in workspace config but none of its four acpmock agent definitions exposed a `model` config option containing that value. Production correctly fails loud when a preferred model is not advertised.
- **Fix commit:** `cfcbdf145` on the rebased `main` baseline already co-ships the advertised fixture model; the Hermes replay preserved that canonical version.
- **Regression test:** existing `TestDaemonE2EGoalCommandsShouldSurviveControlsDisconnectAndRestart`; it failed before the fixture co-ship and passed 6/6 subtests under `-race` after the fix.

## Verification

- **Retested:** 2026-07-12, automated Goal E2E owner · **Report:** docs/qa/reports/2026-07-12-hermes-bridge.md
- **Result:** The exact Goal E2E passed. The full runtime lane then advanced to the separate diagnostics-isolation failure tracked as BUG-20260712-reasoning-evidence-attribution.
