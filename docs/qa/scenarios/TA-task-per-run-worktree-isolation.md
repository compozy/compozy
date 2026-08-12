---
id: TA-task-per-run-worktree-isolation
area: TA
title: Run a task in its enqueue-time per-run worktree
persona: Ada
journey: J-isolated-task-loop-execution
expected: Setting per_run, enqueueing, changing the live profile, and finishing the run leaves the run and its dedicated session bound to the original fresh run branch with identical state through CLI, HTTP, UDS, and worktree reads.
entry_points: compozy task profile set-worktree; compozy task run; PATCH /api/tasks/:id/execution-profile/worktree; compozy__task_worktree_policy_set
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

QA impact: Task 04 adds the task policy patch, enqueue-time snapshot, claimed-run materialization,
session binding, and durable worktree attribution. The Phase C walk must also reverse the edit order:
enqueue under `none`, switch the live profile to `per_run`, and prove the queued run still uses root.
