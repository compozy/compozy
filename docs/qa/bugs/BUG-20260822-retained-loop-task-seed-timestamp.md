# BUG-20260822-retained-loop-task-seed-timestamp: Retention seed writes unreadable task timestamps

- **Status:** verified
- **Impact (user-side):** Blocks-Verification
- **Severity:** Medium · **Priority:** P0
- **Persona Affected:** Release QA operator
- **Journey Step:** Verify a Loop task after its owning run was retained away
- **Scenarios:** Tasks retained Loop record journey
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The retained-record E2E seeder passed `time.Time` values directly to SQLite. The driver serialized
them in a format the production task reader does not accept, so the single-task read failed with
HTTP 500 before the browser could verify truthful degraded provenance.

## Reproduction

- **Invariant:** E2E database seeds use the same RFC3339 timestamp representation as persisted task
  records.
- **Owning layer:** Retained Loop task E2E fixture.
- **Canonical suite:** retained record journey in `web/e2e/__tests__/tasks.spec.ts`.

## Fix

The seeder now writes queued and terminal timestamps as RFC3339Nano strings.

## Verification

The real daemon single-task read succeeds and the retained record Playwright journey passes.

