# BUG-20260822-loop-respond-stdin-e2e-drift: Browser unblocker executor expects obsolete argv payload

- **Status:** verified
- **Impact (user-side):** Blocks-Verification
- **Severity:** Medium · **Priority:** P0
- **Persona Affected:** Loop approver
- **Journey Step:** Answer a live Loop request from the published CLI unblocker
- **Scenarios:** LP-web-needs-you-gate-provenance; LP-web-pending-request-provenance
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The daemon publishes `compozy loop respond ... --payload-stdin`, but the Playwright executor still
required the removed `<json>` argv placeholder. The real CLI journey therefore stopped before it
could answer the request.

## Reproduction

- **Invariant:** The browser journey must execute the exact public unblocker command and deliver its
  JSON answer over stdin when the command declares `--payload-stdin`.
- **Owning layer:** Daemon-served Playwright fixture.
- **Canonical suite:** E2E-013 in `web/e2e/__tests__/loop-run.spec.ts`.

## Fix

The executor now requires `--payload-stdin` and writes the JSON answer to the child process stdin.

## Verification

Focused E2E-013 passed, and the fresh complete daemon-served browser lane passed with 227 tests,
3 declared conditional skips, and 0 failures. The published stdin unblocker completed successfully.
