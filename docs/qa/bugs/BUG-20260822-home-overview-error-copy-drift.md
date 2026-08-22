# BUG-20260822-home-overview-error-copy-drift: Home overview failure heading drifts from its public contract

- **Status:** verified
- **Impact (user-side):** Degrades-Operation
- **Severity:** Low · **Priority:** P1
- **Persona Affected:** Ada, desktop operator
- **Journey Step:** Distinguish an overview read failure from daemon disconnection
- **Scenarios:** Home daemon-served error journey
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The connected Home error state rendered `Couldn't load the overview`, while its canonical browser
contract requires `Unable to load the home overview`. The shorter text lost the surface name and
made the E2E contract fail.

## Reproduction

- **Invariant:** The Home overview error state uses the stable, surface-specific heading while the
  daemon connection remains connected.
- **Owning layer:** Web Home dashboard presentation.
- **Canonical suite:** `web/e2e/__tests__/dashboard.spec.ts`.

**Expected:** `Unable to load the home overview`.
**Actual:** `Couldn't load the overview`.

## Fix

Restored the canonical heading in the production Home dashboard.

## Verification

Real daemon-served Playwright Home error journey: 1 passed in 46.8 seconds.

