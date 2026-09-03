# BUG-20260830-busy-input-turn-end-stranded: A queued prompt can remain stuck when a turn ends

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-13, queue a follow-up while the current answer settles
- **Scenarios:** RT-019
- **Found:** 2026-08-30 · **Report:** docs/qa/reports/2026-08-28-integrated-terminal-rebase.md

## Summary

Théo could submit a follow-up while the current answer was settling, see the authored message in the
thread, and never receive the next answer. Reload preserved the queued input but did not dispatch it.

## Reproduction

- **Charter:** CH-016 · **Tour:** Multi-Tab Tour
- **Environment:** desktop Chromium / real daemon / ACP fixture provider / en-US

1. Start a session and wait until its first assistant answer is visible but its turn is still settling.
2. Submit a second prompt through the Web composer, which accepts it as a queued busy input.
3. Wait for the first turn to become idle.

**Expected:** The durable queued prompt dispatches once after the active turn ends.
**Actual:** The user message remained visible, the session stopped, and the queued prompt never reached
the provider.

## Evidence

- Exact-head CI run `33305085833`, Web E2E job `99240126285`.
- `docs/qa/reports/2026-08-28-integrated-terminal-rebase.md` records the deterministic race replay and
  the public browser retest.

## Fix

- **Root cause:** Busy classification and queue persistence were separate concurrency boundaries. The
  turn-end selector could inspect an empty queue before persistence completed, while the enqueue path
  did not retry the selector after its durable write.
- **Fix commit:** pending; included in the exact-head CI remediation commit
- **Regression test:** `internal/session/manager_busy_input_test.go` blocks both anonymous and
  identity-bearing queue persistence until the turn-end selector observes an empty queue, then proves
  the persisted entry dispatches exactly once. Both variants passed 10/10 under `-race`.

## Verification

- **Retested:** 2026-08-30, same persona/journey · **Report:**
  `docs/qa/reports/2026-08-28-integrated-terminal-rebase.md`
- **Result:** The Skills marketplace browser journey delivered the second prompt and rendered the
  second acknowledgement. The broader CH-016 live-provider and multi-tab charter remains separately
  tracked by RT-019's `blocked-verify` verdict.
