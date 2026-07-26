# BUG-20260713-task-approval-duplicates-open-run: Approving a pre-enqueued task tries to create a second run

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Approver
- **Journey Step:** Approve a human-in-the-loop Task whose first run is already pending
- **Scenarios:** TA-041
- **Found:** 2026-07-14 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** AGH-71 live approval replay

## Summary

Task creation with human-in-the-loop approval already enqueues one gated run. Approving that Task first made it Ready and then attempted to enqueue a second run. The UI reported an invalid-status error even though the approval state changed, leaving the user unable to tell whether execution would start. An intermediate correction reused the run but attempted to alias the approval idempotency key across different operation origins, producing a second error.

## Reproduction

1. Create a workspace Task with Agent/pool ownership, human-in-the-loop approval, and enqueue-on-create.
2. Confirm the detail shows `Blocked`, `Awaiting approval`, and `Runs 1`.
3. Open Inbox and approve the Task.
4. Inspect the toast, Task status, run count, run ID, and eventual task-role worker.

**Expected:** Approval makes the existing gated run claimable. The Task retains one run and one attempt, and the worker claims that same run without an error.
**Actual:** The original path attempted a second enqueue after the Task became Ready and returned an invalid transition. The first correction then returned an idempotency-origin mismatch for the existing run.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/task-approval-reuses-open-run-fixed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/task-approval-reuses-open-run-fixed.json`

## Fix

- **Root cause:** Approval assumed it always owned run creation. It did not distinguish a pre-enqueued approval-gated run from a Task whose approval transition must auto-enqueue its first run. The first reuse attempt also treated an existing run from `tasks.enqueue_run` as eligible for a new `tasks.approve` idempotency alias, violating origin ownership.
- **Correction:** After approval, the Task service lists the Task's runs and reuses the sole nonterminal run after validating ownership. It auto-enqueues only when no open run exists, and it saves an approval idempotency alias only for a run created by the approval operation itself.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** The canonical Task manager unit and integration approval suites prove the returned run is the original run, `ExistingRun` is true, exactly one run exists, and the approved run remains claimable.

## Verification

- In-app Browser Task `task-e316e5733fb4feb0` retained only `run-2ea4ee28ec4b2236`; Inbox approval rendered `Task approved.` with no invalid-status or idempotency error. Cursor/Grok session `sess-23117c8e3aad8ea6` claimed and completed that exact run.
- Independent Task `task-d702248d032de117` retained only `run-09474581eb43d3d3`; session `sess-d84ebf495d1547f6` claimed it, and a real reload rendered both Task and original run Completed.
