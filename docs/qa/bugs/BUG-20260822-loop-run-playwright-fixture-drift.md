# BUG-20260822-loop-run-playwright-fixture-drift: Loop run browser fixtures no longer establish their contracts

- **Status:** verified
- **Impact (user-side):** Blocks-Verification
- **Severity:** Medium · **Priority:** P0
- **Persona Affected:** Release QA operator
- **Journey Step:** Verify terminal outcomes, graph states, story paging, and usage
- **Scenarios:** LP-web-run-default-read-briefing; LP-web-run-operator-register
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

Several seeds in the canonical Loop run Playwright file no longer established the state their
assertions owned: quarantine is now intentionally non-terminal, a route duplicated its default and
referenced an undeclared input, the retry agent reported no usage, and the story selector also
matched nested related-run links.

## Reproduction

- **Invariant:** Each daemon-served browser seed must establish the public state named by its case,
  and selectors must identify only the surface element that owns the assertion.
- **Owning layer:** Daemon-served Playwright fixtures.
- **Canonical suite:** E2E-014 through E2E-018 in `web/e2e/__tests__/loop-run.spec.ts`.

## Fix

Terminal-failure cases use a one-round exhausted run, the route declares its input without a
duplicate destination, the retry fixture reports deterministic usage, and story beats use an exact
test-id matcher.

## Verification

The complete daemon-served canonical Loop run file passed 17/17, including strict E2E-019. After
the remaining deterministic fixture failures were repaired, the fresh broader Web lane passed with
227 tests, 3 declared conditional skips, and 0 failures.
