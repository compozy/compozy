---
id: TA-task-fanout-worktree-isolation
area: TA
title: Isolate every designated fan-out run in its own worktree
persona: Bruno
journey: J-isolated-task-loop-execution
expected: Fan-out with worktree-per-run creates one attributable run branch and ready worktree per designation, keeps sibling identities distinct, and lets one failed or canceled run leave the other runs unaffected.
entry_points: compozy task fan-out --worktree-per-run; POST /api/tasks/:id/runs/fan-out; compozy__task_fanout_runs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

QA impact: Task 04 adds the fan-out isolation override and per-run event attribution. The Phase C
walk must inspect all run, session, worktree, and `worktree.created` identities, then fail or cancel
one sibling and confirm the remaining siblings continue independently.
