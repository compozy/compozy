---
id: TA-worktree-web-task-policy
area: TA
title: Set a task's worktree policy from the setup sheet
persona: Bruno
journey: J-isolated-task-loop-execution
expected: The task setup sheet exposes the worktree policy in the Environment fieldset with the locked mode vocabulary, offers only same-workspace ready worktrees for a named reference, flags a reference that no longer resolves, and locks every control while a run is active. The policy is written through its own patch route so saving unrelated setup fields never overwrites it, and the profile view reads the policy back.
entry_points: S10 Task detail -> Setup -> Environment -> Worktree
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/web-fanout-fixed-task.json; web/src/systems/tasks/components/__tests__/task-setup-dialog.test.tsx
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: TA-task-per-run-worktree-isolation; TA-task-fanout-worktree-isolation
---

QA impact: Task 07 adds the task worktree policy fieldset, its read row, and the patch-only write path.
