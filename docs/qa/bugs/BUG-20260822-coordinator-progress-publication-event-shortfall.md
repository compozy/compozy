# BUG-20260822-coordinator-progress-publication-event-shortfall: Concurrent Loop progress loses its publication wake

- **Status:** verified
- **Impact (user-side):** Blocks-Operation
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada, autonomous Loop operator
- **Journey Step:** Observe a wide fan-out progressing without follow-up prompts
- **Scenarios:** LP-web-timeline-graph-rows
- **Found:** 2026-08-22 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

When task progress landed while a coordinator run was active, the store could append a
concurrent-progress wake to the committed run list. The task service reserved event IDs only for
authored node runs and the planned next coordinator, so publication failed with one fewer ID than
committed runs. Wide fan-outs stalled even though their workers had completed.

## Reproduction

- **Invariant:** Every run committed by coordinator completion has an event ID reserved before the
  completion transaction begins.
- **Owning layer:** Task service coordinator publication lifecycle.
- **Canonical suite:** `TestManagerStartRunShouldExecuteCoordinatorInDaemonWithoutSession`.

Run the wide fan-out browser journey and inspect the daemon log while workers settle.

**Expected:** Concurrent progress schedules and publishes one reconciliation wake.
**Actual:** Publication reports `coordinator publication has N runs but only N-1 reserved event IDs`
and autonomous progress stalls.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-20260822-025427-168223-lab/qa-artifacts/qa/diagnostics/fanout-event-reservation-daemon.log`

## Fix

Coordinator publication now pre-reserves one additional identity when an in-flight generation can
produce the store-owned concurrent-progress wake and no explicit post-commit wake suppresses it.

## Verification

- Focused canonical Go test: green, including the new possible-wake reservation case.
- Real daemon-served Playwright fan-out: 1 passed in 58.9 seconds; 120 workers settled without a
  follow-up prompt.

