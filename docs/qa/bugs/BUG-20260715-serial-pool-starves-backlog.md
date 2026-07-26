# BUG-20260715-serial-pool-starves-backlog: Busy worker pool parks compatible queued work

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Sofia Mendes; registered collaborator agents; Operator
- **Journey Step:** Northstar Pay delivery, serial frontend pool
- **Scenarios:** RT-073; TA-044
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

Later runs assigned to a single compatible worker were parked as `needs_attention` while that worker
was successfully completing earlier runs. The queued work could not complete even though the required
capacity existed and was making progress.

## Reproduction

- **Charter:** Marketplace Northstar playbook · **Tour:** autonomous one-kickoff delivery
- **Environment:** Isolated AGH lab with one shared workspace, one frontend pool worker, and real Claude providers

1. Queue three or more runs owned by the same single-session worker pool.
2. Let the worker claim and process the first run.
3. Keep the later compatible runs queued across at least 10 scheduler cycles.
4. Inspect the later run and scheduler status while the first run remains active.

**Expected:** Compatible busy capacity keeps later runs queued without advancing their escalation
budget. When the active lease completes, the same worker can claim the next run.
**Actual:** Busy capacity was excluded from wake selection and then treated as worker absence. Run
`run-062c70a3b7273704` reached `needs_attention` while the frontend worker was processing its serial
backlog.

## Evidence

- Isolated lab: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-final-pass-20260715-20260715-160444-641330-lab`.
- Affected task/run: `task-northstar-pay-004` / `run-062c70a3b7273704` at `2026-07-15T16:19:10`.
- Mandatory teardown: `qa-artifacts/qa/teardown.json` records `clean=true` with no surviving lab process.

## Fix

- **Root cause:** Scheduler selection combined structural compatibility with momentary availability.
  Busy, starting, prompting, and same-cycle-reserved sessions disappeared from the eligible set, so
  convergence interpreted a saturated compatible pool as an absent pool.
- **Correction:** Scheduler capacity classification now separates available, capacity-waiting,
  unmatched, and indeterminate runs. Capacity-waiting and indeterminate runs freeze any durable
  escalation episode; only true absence or available-but-unclaimed capacity advances convergence.
  Public `starved_run_count` now counts active durable escalation episodes instead of raw queue age.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** `internal/scheduler/scheduler_test.go` owns disposition and budget behavior;
  `internal/scheduler/scheduler_integration_test.go` proves a real serial backlog survives more than
  10 cycles and becomes claimable after the active lease completes.

## Verification

- The canonical regression failed before the correction with busy, prompting, and starting sessions
  incrementing `NoMatchRuns` and `StarvedRuns`.
- Focused scheduler, daemon projection, task status, GlobalDB, and scheduler integration suites pass
  under `-race`.
- Fresh isolated one-kickoff provider retest `marketplace-northstar-capacity-final-20260715-20260716-001326-274237` passed. One frontend session processed tasks 001, 002, and 004 serially; the queued compatible work remained `capacity_waiting`, public starvation/attention counts stayed zero, and the same session claimed the next run after its active lease completed. No elastic worker appeared. All 12 root tasks completed during the 30-minute observation window; teardown is `clean=true` with no survivors.
