# BUG-20260713-task-role-dispatch-repeats: One queued run repeatedly re-prompts the same task-role session

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree, wait for a task-role worker to claim one queued run
- **Scenarios:** TA-task-role-session-activation; TA-parent-rollup-completion
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** p26 fixed-path live Cursor replay

## Summary

The repaired activation path sends the first correlated synthetic turn, but when the real Cursor worker responds without claiming the run, later scheduler cycles send the same synthetic task/run notification again. One active system session accumulated 14 assistant responses for the same `run-be2c1d6592e2c043` without a user prompt, Web recovery, or replacement run. This spends provider time and tokens, floods the transcript, and still leaves the run queued/unowned.

## Reproduction

1. Recover a queued task run so starvation recovery creates one task-role Cursor session.
2. Let the initial correlated notification reach the provider.
3. Have the provider return without claiming the run; the live case was blocked by Cursor Ask mode.
4. Leave the run queued and revisit the same session transcript after scheduler cycles.

**Expected:** At most one initial activation prompt is delivered for a `(session_id, run_id)` assignment. A completed turn that does not claim remains observable and requires an explicit recovery/re-dispatch policy; scheduler polling does not spend the provider again.
**Actual:** The same session produced 14 assistant responses about the same run. The first response took 21 seconds; 13 later responses repeated the Ask-mode blocker with no user action.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-role-repeated-synthetic-turns.dom.txt`
- Session `sess-1e9a13013651c8b0`; task `task-f6638f9897b1b0f8`; run `run-be2c1d6592e2c043`; channel `coord-run-be2c1d6592e2c043`.
- In-app Browser count on the fresh transcript: 14 `Use as Goal` actions and 15 visible Ask-mode references, with no active composer/user turns between the repeated responses.

## Fix

- **Root cause:** `drainTaskRolePromptEvents` removed the in-memory `promptInFlight` marker when the synthetic turn channel closed. Because the durable Task run remained queued/unclaimed, the next starvation sweep treated the same session/run assignment as never activated and dispatched the initial prompt again.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** The canonical daemon task-role runtime suite derives a deterministic synthetic turn ID from `(session_id, run_id)` and uses persisted terminal events as the durable idempotency proof. It covers one completed initial activation, repeated scheduler sweeps, supported new-run/session activation, and retryability after synchronous startup failure without retaining settled assignments indefinitely in memory.

## Verification

- Fresh child A bound one run to only `sess-0bb0f23ac1414396`; fresh child B bound one run to only `sess-64f9badf5a65dd2f`. Each real Cursor/Grok worker received one activation, claimed once, and completed once.
- Two additional approval-gated controls retained one run before and after approval, then bound one real task-role session each and completed. No repeated assistant turns, duplicate run, or second session appeared.
- Browser evidence: `agh71-faithful-child-b-one-run.dom.txt` and `task-approval-reuses-open-run-fixed.{dom.txt,json}` under the active post-onboarding-fix lab.
