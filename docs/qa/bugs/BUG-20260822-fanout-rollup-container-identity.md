# BUG-20260822-fanout-rollup-container-identity: Fan-out rollup uses the worker step identity

- **Status:** verified
- **Impact (user-side):** Degrades-Operation
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada, Loop run inspector
- **Journey Step:** Locate a fan-out and read its aggregate progress
- **Scenarios:** LP-web-timeline-graph-rows
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The roster projected a fan-out rollup under the repeated worker step instead of the authored
fan-out container. The graph therefore could not attach the aggregate to the entity an operator
sees in the Loop definition.

## Reproduction

- **Invariant:** A fan-out aggregate is named and keyed by its authored container while worker rows
  remain folded into that aggregate.
- **Owning layer:** Loop roster projection.
- **Canonical suite:** `TestRosterContract`.

**Expected:** A `revisores` fan-out targeting `revisar` produces one `revisores` rollup.
**Actual:** The aggregate is returned as `revisar`.

## Fix

Roster assembly now resolves direct fan-out branch targets back to their authored container,
including nested graphs, and omits the control row when worker rows provide the aggregate.

## Verification

- Focused `TestRosterContract` fan-out identity case: green.
- Real daemon-served Playwright fan-out: the `revisores` entity renders one 120-item rollup.

