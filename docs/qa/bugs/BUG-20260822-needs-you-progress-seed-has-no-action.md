# BUG-20260822-needs-you-progress-seed-has-no-action: Needs-you browser seed has no action progress

- **Status:** verified
- **Impact (user-side):** Blocks-Verification
- **Severity:** Medium · **Priority:** P0
- **Persona Affected:** Loop operator
- **Journey Step:** Read a needs-you run without opening Inspect
- **Scenarios:** LP-web-run-default-read-briefing
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The canonical browser seed created only an approval gate and an ask node. Both are control nodes, so
the daemon truthfully served a zero-step round while the journey required visible `Step N of M`
action progress.

## Reproduction

- **Invariant:** The default-read browser journey must exercise at least one real action step before
  asserting action-step progress.
- **Owning layer:** Daemon-served Playwright fixture.
- **Canonical suite:** E2E-012 in `web/e2e/__tests__/loop-run.spec.ts`.

## Fix

The seed now includes a real transform action alongside the two concurrent human blockers.

## Verification

Focused E2E-012 passed, and the fresh complete daemon-served browser lane passed with 227 tests,
3 declared conditional skips, and 0 failures. The needs-you journey exposed real action progress.
