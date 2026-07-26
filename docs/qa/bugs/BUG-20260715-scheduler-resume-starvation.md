# BUG-20260715-scheduler-resume-starvation: Global pause time exhausts queued-run escalation after resume

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Sofia Mendes; registered collaborator agents; Operator
- **Journey Step:** Northstar Pay activation barrier release
- **Scenarios:** TA-047; RT-073
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

Queued runs accumulated starvation age while the scheduler was globally paused. Resuming after a deliberate 5m40s activation barrier made every queued run immediately eligible for convergence. Secondary tasks in a single-agent owner pool then exhausted all 10 escalation cycles while that agent was legitimately working on its first task and were parked as `needs_attention` before they could be claimed.

## Reproduction

1. Globally pause the scheduler.
2. Queue two or more runs owned by the same single-agent pool.
3. Keep the activation barrier paused longer than `autonomy.scheduler.min_queued_age`.
4. Resume the scheduler and let the pool agent claim the first run.
5. Observe the remaining queued run across the next 10 scheduler cycles.

**Expected:** Time behind the global pause does not count toward starvation. After resume, each queued run must remain dispatch-eligible for `min_queued_age` before its convergence budget advances.
**Actual:** The original `queued_at` included the paused interval. The remaining run entered convergence on the first resumed cycle and reached `needs_attention` after 10 cycles even though its only matching worker was still active.

## Evidence

- Isolated lab: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-final-20260715-20260715-152506-977392-lab`.
- Activation evidence: `qa-artifacts/qa/task-activation.json` and `journey-log.jsonl` show all 12 runs were queued behind the scheduler barrier at `2026-07-15T15:29:24Z`.
- The one-kickoff barrier was confirmed and released at `2026-07-15T15:35:04Z`; tasks `task-northstar-pay-001`, `002`, `004`, and `008` later reported `run queued 8m4s without a claim after 10 escalation cycles`.
- Mandatory teardown: `qa-artifacts/qa/teardown.json` records `clean=true`, no survivors, and completion at `2026-07-15T15:38:03Z`.

## Fix

- **Root cause:** Scheduler pause prevented dispatch and convergence cycles, but starvation selection and diagnostics always measured `now - run.queued_at`. The durable scheduler control row already records the resume transition in `scheduler_pause.updated_at`, but the scheduler discarded that field and retained only the boolean pause state.
- **Correction:** Scheduler cycles now retain the full persisted pause state and calculate one effective starvation age from `max(run.queued_at, scheduler_pause.updated_at)`. Selection, durable convergence, starved events, and needs-attention diagnostics share that calculation. Real queue timestamps, ordering, and existing escalation budgets remain unchanged.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical scheduler pause suite proves a pre-pause queued run does not create or advance a starvation budget before the post-resume minimum age, then advances exactly once after that boundary. The scheduler is constructed only from the persisted resume timestamp, covering daemon restart semantics.

## Verification

- The regression failed before the production correction with `StarvedRuns = 1, want 0 while resume grace is active`.
- `CGO_ENABLED=1 go test -race ./internal/scheduler/... -count=1` passes.
- Race-enabled CLI, API core, and scheduler owner suites pass together.
- Scoped `golangci-lint` for CLI, API core, and scheduler returns zero issues.
- Fresh isolated one-kickoff provider retest `marketplace-northstar-capacity-final-20260715-20260716-001326-274237` passed the real scheduler barrier: all 12 root tasks completed after one confirmed kickoff and one resume, with no premature starvation or needs-attention projection. The full 30-minute observer completed without stall; teardown is `clean=true` with no survivors.
