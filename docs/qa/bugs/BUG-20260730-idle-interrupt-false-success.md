# BUG-20260730-idle-interrupt-false-success: Idle sessions reported a successful interrupt

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-13 control a prompt, step 4
- **Scenarios:** RT-019
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Théo could interrupt an idle session and receive success even though no prompt was running.

## Reproduction

- **Charter:** CH-session-busy-input-truth · **Tour:** Garbage Tour
- **Environment:** isolated current-source daemon / native Codex session / HTTP / en-US

1. Create a session and wait until it is `idle` with `active_prompt=false`.
2. POST the session interrupt endpoint.

**Expected:** HTTP 409 states that no prompt is in progress.
**Actual:** The pre-fix session manager returned success.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** Busy-input interrupt handling did not require an active prompt before calling the provider interrupt path.
- **Fix commit:** bd0617c
- **Regression test:** `internal/session/manager_busy_input_test.go`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** The idle public session returned HTTP 409 and remained healthy and idle.
