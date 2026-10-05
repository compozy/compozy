# BUG-20261005-loop-initial-gate-route-lost: A gate at the start of a Loop loses its continuation route

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, observe a bounded unsuccessful outcome
- **Scenarios:** LP-003; LP-revise-repair-context
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

A valid Loop beginning with a command gate records its rejected verdict and the authored
next_generation route, then ends Failed/contract without creating that next generation.
The same gate progresses when an action precedes it.

## Reproduction

CH-012, Bruno, desktop/fast Wi-Fi/en-US. Validate and create studio-release-readiness with a
single command gate checking the missing onboarding/release-approval.md artifact, fixed_passes,
and on_result.fail=next_generation. Start it through the public CLI and open the returned Web
URL. Independently repeat through HTTP using studio-initial-release-gate and read through UDS.

Expected: the declared continuation is admitted subject to iteration and no-progress limits.
Actual: generation 1 stores gate_route_next_generation, but the run immediately ends failed.

## Evidence

CLI run looprun-31a3dbd1fc7919f5 and independent HTTP run looprun-6add25b42f600c0d both end
Failed in generation 1. Their rejected gate payload records next_generation and the stable
command_expectation_failed blocker. The producer-present control looprun-8168d4297043aafc
advances to generation 8; its separate missed no-progress bound is tracked independently.

Receipts under docs/qa/evidence/2026-10-02-untested/: loops-stalled-candidate-validate.json,
loops-stalled-readiness-{start,why,http,web-first}.json,
loops-initial-gate-replay-{start,uds,owned}.json and loops-initial-gate-diagnosis.json.
The seven-frame loops-stalled-readiness-bruno recording is closed. No model or mocked service
is involved; the real command criterion reads the real laboratory workspace.

## Fix

Root cause: the initial control plan collects the route-causing gate verdict, but
finishInitialControlPlan selects noReadyNodesTerminal instead of the existing idle-generation
finisher that owns gate succession. The canonical coordinator suite must cover a gate evaluated
before any task dispatch, with its verdict retained and a bounded successor admitted.

Fix commit: pending. Production repair and focused/race checks pass; required delivery gate remains. Repair the owning initial planning path and re-walk
both root-gate and producer-present definitions without changing the authored gate requirement.

## Functional replay

The initial gate now reaches generation 2 in looprun-8526037f82686824. Both this run and
the producer-present looprun-59d3f650509769cc end Stalled/no_progress at window 2, with
gate_next_generation provenance and no generation 3. The independent revise control
looprun-8001f55521e769f8 also stalls at window 2 with gate_revise provenance. All use the
real workspace command, retain rejected verdicts and consume zero model tokens.

CLI/HTTP/UDS agree; fresh production Web and inspected screenshots display the same
terminal state and round count. The loops-gate-routing-fixed-bruno recording closes with
eleven frames. All internal/loop/... race packages pass after preserving successful
control-only completion. See loops-gate-routing-replay-summary.json and the current report.
Status remains open until the enclosing delivery gate and commit.
