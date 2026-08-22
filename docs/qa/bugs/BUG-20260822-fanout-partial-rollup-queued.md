# BUG-20260822-fanout-partial-rollup-queued: Partial fan-out progress renders as queued

- **Status:** verified
- **Impact (user-side):** Degrades-Operation
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada, Loop run inspector
- **Journey Step:** Read live fan-out progress in Inspect graph
- **Scenarios:** LP-web-timeline-graph-rows
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

A fan-out with completed workers and queued siblings rendered as queued because queued state took
precedence over evidence of completed work. The graph hid live progress that public reads already
reported.

## Reproduction

- **Invariant:** A non-terminal fan-out with `0 < done < total` is running, even when its remaining
  workers are queued.
- **Owning layer:** Web Loop DAG view model.
- **Canonical suite:** `web/src/systems/loops/lib/__tests__/loop-run-dag-view.test.ts`.

**Expected:** A 1-of-2 rollup renders running and its incoming gutter is live.
**Actual:** It renders queued.

## Fix

The fan-out band now classifies partial completion as running before considering queued remainder.
The rendered aggregate also exposes its count and summary through an accessible name.

## Verification

- Focused Loop DAG view-model test: green.
- Real daemon-served Playwright fan-out: live edge pulse and 120-item rollup both observed.

