# BUG-20260826-session-delete-return-race: Session deletion can leave the browser on another session

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-15, delete an open session
- **Scenarios:** RT-014
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

After deleting the open session, the daemon could reconcile its window before the browser finished
the local delete transition. The browser then focused another session instead of returning to the
deleted session's agent page, even though the DELETE request succeeded and the session was gone.

## Reproduction

- **Charter:** full daemon-served Web E2E · **Tour:** Regression Tour
- **Environment:** isolated Playwright runtime / Chromium / en-US

1. Open a session while another session remains available for the same agent.
2. Stop the active turn and confirm **Delete session**.
3. Let daemon window reconciliation remove the deleted session window before the local effect runs.
4. Observe the route after the DELETE request returns 204.

**Expected:** The browser returns to the deleted session's agent route.
**Actual:** The browser remains on the surviving session selected by shell reconciliation.

## Evidence

- `.tmp/playwright/test-results/__tests__-session-hardenin-1a202-the-session-across-surfaces/`
- The captured network log records DELETE 204 and a subsequent 404 for the deleted session while the
  screenshot and error context show the surviving session route.

## Fix

- **Root cause:** `returnToAgent` treated `coordinator.userClose(windowId) === false` as a navigation
  failure. That result also means the daemon already removed the window, so skipping `userOpen`
  discarded the required agent-route transition.
- **Fix:** Attempt the close for idempotent cleanup, then always open the agent route after the close
  attempt settles.
- **Regression test:**
  `web/src/systems/os/apps/session/__tests__/session-window.test.tsx`, including the daemon-first
  `userClose === false` race.

## Verification

- **Focused unit:** 15/15 session-window controller tests passed on 2026-08-26.
- **Browser retest:** The exact session-hardening deletion scenario passed against the rebuilt
  production bundle, then the canonical `make test-e2e-web` lane passed 253 tests with 3 intentional
  skips and zero unexpected or flaky results.
