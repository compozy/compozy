# BUG-20260713-stopped-session-goal-clear-fails: Stopped sessions expose a Goal control that cannot clear

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-26, revisit and clear a blocked Goal after its session stops
- **Scenarios:** GL-019
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Post-fix live Goal lifecycle replay

## Summary

A durable blocked Goal remains attached when its owning session is stopped, and the session UI still renders `Clear goal` as an actionable control. Clicking it fails with the internal lifecycle error `session is not active`, leaves the Goal attached, and gives the operator no supported recovery path.

## Reproduction

1. Open stopped session `sess-84c5282a292e7f0f`, which retains blocked Goal run `looprun-e6830bc6fd4a086f`.
2. Confirm the Goal status region renders `Clear goal`.
3. Click `Clear goal`.
4. Observe the toast and reload the Goal status.

**Expected:** An exposed Clear control durably clears the attached Goal regardless of the stopped runtime process, or the UI truthfully withholds the control and presents the supported recovery action.
**Actual:** The control calls the active-session path, returns `session is not active ... (stopped)`, and leaves the Goal blocked and attached.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/goal-stopped-session-clear-fails.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/goal-stopped-session-clear-fixed.dom.txt`
- Session `sess-84c5282a292e7f0f`; Goal run `looprun-e6830bc6fd4a086f`.

## Fix

- **Root cause:** `session.Manager.SendPrompt` required active prompt admission before it parsed and dispatched Goal commands, so `/goal clear` failed at `lookupPromptSession` and never reached the already-durable daemon/store handler.
- **Fix commit:** uncommitted QA remediation batch
- **Regression test:** The canonical Session Manager command-admission suite creates and stops a real session, dispatches `/goal clear` from durable session metadata, requires the exact workspace/session/caller identity, returns `GoalOutcomeCleared`, and proves zero ACP prompt calls. Existing daemon and tagged GlobalDB suites retain stopped-origin validation, Goal tombstone/projection, and historical run/turn preservation.

## Verification

- Full `internal/session` race, canonical daemon Goal lifecycle, tagged GlobalDB clear tombstone/projection integration, new-diff lint, formatting, diff check, caps, and deslop passed.
- Same-persona Browser replay passed after rebuilding the isolated daemon. One click removed the Goal status/control without a failure toast, and the preserved Run page still rendered all four historical Goal turns and their evidence.
