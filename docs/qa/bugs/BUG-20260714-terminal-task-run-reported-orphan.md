# BUG-20260714-terminal-task-run-reported-orphan: Inspect reports a completed run as an active orphan

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** Inspect a completed task-role run after its worker session stops
- **Scenarios:** TA-terminal-run-inspect
- **Found:** 2026-07-14 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** AGH-71 verification

## Summary

The Task and run are durably Completed, but Task inspect still emits the live error `task_run_orphan` because the completed run retains an ownership token and its bound session is terminal. The suggested `agh task release ... --reason "orphaned"` command is invalid recovery guidance for terminal work and contradicts the Task's truthful terminal state.

## Reproduction

1. Let a real task-role session claim and complete one Task run.
2. Wait for the session to stop.
3. Reload the completed Task detail and inspect diagnostics.

**Expected:** A terminal run emits no active orphan diagnostic or recovery command.
**Actual:** Completed `run-df8c1dd9a1b8b5f8` is reported as `task_run_orphan` because stopped session `sess-64f9badf5a65dd2f` remains bound.

## Evidence

- Task `task-1f83323b5632a917`; run `run-df8c1dd9a1b8b5f8`; session `sess-64f9badf5a65dd2f`.
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/agh71-faithful-child-b-one-run.dom.txt`

## Fix

- **Root cause:** Task inspect used the preserved truncated claim-token hash plus terminal session state as sufficient proof of an orphan. Fenced completion intentionally preserves the hash for audit/idempotency after clearing the raw token and lease, so terminal history satisfied that incomplete predicate.
- **Correction:** `detectInspectOrphanRun` excludes every canonical terminal run status before evaluating token/session evidence. Claimed/open runs retain the existing orphan detection; failed runs may still emit their separate crash diagnostic.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `TestInspectTaskDiagnostics/Should not emit orphan diagnostic for terminal run statuses` covers completed, failed, and canceled runs while the existing claimed-run case proves the live orphan remains detectable.

## Verification

- Rebuilt daemon PID `19957` from the corrected source and reloaded the original Task and run through the in-app Browser.
- Both pages render `Completed`, bound stopped session `sess-64f9badf5a65dd2f`, `Next action terminal`, zero diagnostics, and no release command. Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/terminal-task-run-no-orphan-fixed.dom.txt`.
