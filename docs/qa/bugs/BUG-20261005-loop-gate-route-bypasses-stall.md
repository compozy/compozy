# BUG-20261005-loop-gate-route-bypasses-stall: Gate retries ignore the no-progress bound

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-01, stop repeated work that makes no progress
- **Scenarios:** LP-003; LP-revise-repair-context
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

A gate-driven next_generation route keeps repeating an unchanged blocker beyond the run's
effective no-progress window. The Loop spends its entire iteration cap and reports Exhausted
instead of stopping as Stalled when the repeated blocker reaches the configured bound.

## Reproduction

CH-012, Bruno, desktop/fast Wi-Fi/en-US. Publish studio-release-readiness version 2 with a
prepare_packet transform followed by a command gate checking the missing release-approval
artifact. Route gate failure to next_generation. Start through HTTP with no_progress_window=2,
iteration_cap=8 and budget_wall_sec=120. Read the generations through UDS and refresh the Web.

Expected: the unchanged blocker reaches the two-generation bound and ends Stalled/no_progress.
Actual: eight generations repeat command_expectation_failed, then iteration_cap ends Exhausted.

## Evidence

Run looprun-8168d4297043aafc pins no_progress_window=2. Every generation stores the same failed
release_evidence output, blocker signature and next_generation route. CLI why reports
exhausted/iteration_cap at 2026-10-05T11:39:12.907782Z. Fresh Web agrees with eight rounds and
the independent UDS read. This probe uses real command execution and consumes no model tokens.

Receipts under docs/qa/evidence/2026-10-02-untested/:
loops-stalled-with-producer-{http-start,uds,why,web}.json,
loops-stalled-with-producer-terminal.png and loops-gate-no-progress-triage.json.
The terminal screenshot was visually inspected; the seven-frame
loops-stalled-readiness-bruno recording is closed.

## Fix

Root cause: buildFailedGenerationPlan returns through buildGateSuccessionPlan before invoking
terminalForFailedGeneration, which owns the repeated-blocker guard. The existing coordinator
suite covers repeated synthetic failed outputs but misses actual gate-driven continuation.
Extend that suite with real route decisions and a changed-signature continuation control.

Fix commit: pending. Production repair and focused/race checks pass; required delivery gate remains. Preserve explicit route semantics, parked work and
iteration ceilings while applying the established no-progress guard before successor admission.

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
