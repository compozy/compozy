---
id: TA-task-per-run-worktree-isolation
area: TA
title: Run a task in its enqueue-time per-run worktree
persona: Ada
journey: J-isolated-task-loop-execution
expected: Setting per_run, enqueueing, changing the live profile, and finishing the run leaves the run and its dedicated session bound to the original fresh run branch with identical state through CLI, HTTP, UDS, and worktree reads; a denied, failed, cancelled, or stale-lease materialization leaves no branch, directory, registry row, binding, or created event.
entry_points: compozy task profile set-worktree|update; compozy task run; compozy task list --worktree; HTTP/UDS PATCH /api/tasks/:id/execution-profile/worktree; GET /api/tasks|/api/observe/tasks/dashboard|inbox?worktree=; compozy__task_worktree_policy_set; compozy__task_execution_profile_set.worktree; compozy__task_list.worktree
qa_status: pass
bug_ids: BUG-20260813-native-claim-skips-run-start
fix_status: fixed
retest_status: pass
fix_commits: e59a03b6
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/native-handoff-fixed-task.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-native-handoff-fixed.png; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps:
---

QA impact: Task 04 adds the task policy patch, enqueue-time snapshot, claimed-run materialization,
session binding, and durable worktree attribution. The Phase C walk must also reverse the edit order:
enqueue under `none`, switch the live profile to `per_run`, and prove the queued run still uses root.
It must also provoke denial or bootstrap failure and prove the shared creation rollback removes every
artifact before a retry.
