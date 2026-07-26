# BUG-20260713-needs-attention-recovery-hidden: Web hides recovery for an active needs-attention run

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree, recover an escalated child run
- **Scenarios:** TA-033; TA-parent-rollup-completion
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** AGH-71 integrated replay

## Summary

Run `run-0dc2db2a608bf620` is durably `Needs Attention` after ten unclaimed escalation cycles. The run detail truthfully shows that status and `No bound session`, but exposes no recovery action. Its Task detail simultaneously renders the Task itself as `Ready`, shows the needs-attention run as the active run, and offers `Start run` instead of `Recover`.

Activating `Start run` does not create a continuation because the needs-attention run is still active. The supported runtime recovery contract already exists as `POST /api/runs/:id/recover` and CLI `task recover`; it is not reachable from the Web journey.

## Reproduction

1. Let a queued run reach `needs_attention` through scheduler starvation.
2. Open `/tasks/<task-id>/runs/<run-id>` and inspect header actions and diagnostics.
3. Return to the Task detail and inspect the active run plus primary action.
4. Activate `Start run`, then inspect the run count and active run again.

**Expected:** The Task and run details expose one `Recover` action for the active needs-attention run. Recovery terminalizes that run and queues one continuation, with pending/idempotent feedback.
**Actual:** Neither detail exposes `Recover`. Task detail shows an ineffective `Start run`; the existing run remains active and no continuation is created.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-run-needs-attention.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-needs-attention-recovery-hidden.dom.txt`
- Task `task-f6638f9897b1b0f8`; active run `run-0dc2db2a608bf620` remained attempt 1 after `Start run`.

## Fix

- **Root cause:** Web recovery was task-status-only, while runtime truth may be Task `Ready` plus an active run `needs_attention`. The Task lifecycle therefore fell through to `Start run`, run detail had no run-recovery adapter/action, and cache invalidation did not own the run-to-Task continuation transition.
- **Correction:** Task and run details now share the runtime run-recovery mutation, project active-run `needs_attention`, suppress `Start run` for every open run, prevent duplicate submission while pending, and invalidate the owning Task/run plus list, timeline, tree, dashboard, inbox, and scheduler aggregates. A shared predicate exposes Recover only when `attempt < max_attempts`; an exhausted run exposes neither Recover nor Start run.
- **Fix commit:** pending
- **Regression test:** Canonical formatter, Task header, run header, adapter, mutation, Task-page, and run-page suites cover transport, projection, cache invalidation, pending state, Task-versus-run dispatch, and attempt-budget eligibility.

## Verification

- **Retested:** 2026-07-13T10:51Z → 2026-07-13T10:58Z through the original in-app browser and isolated live daemon.
- Exhausted `run-0dc2db2a608bf620` at attempt 1/max attempts 1 rendered `Needs attention` with neither Recover nor Start run. Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-needs-attention-exhausted-fixed.dom.txt`.
- Bruno edited the Task through the UI to max attempts 3. Recover appeared while Start run remained hidden; a single click terminalized the old run and queued exactly one attempt-2 continuation, `run-be2c1d6592e2c043`.
- The fresh Task projection showed two total runs, the new run as the only active Pending run, a new coordination channel, and no duplicate Recover action. Evidence: `ch-task-needs-attention-recoverable-fixed.dom.txt` plus live Task snapshot.
- Deterministic recoverable/exhausted Task and run captures are under `.codex/evidence/needs-attention-recovery-hidden/`; their `teardown.json` is clean.
