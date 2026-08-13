---
id: TA-worktree-web-fanout-isolation
area: TA
title: Isolate fan-out runs from the fan-out dialog
persona: Bruno
journey: J-isolated-task-loop-execution
expected: The fan-out dialog offers per-run isolation, off by default, and states how many worktrees the current assignment lines would create using the same count the request carries. After an isolated fan-out the dialog reports each run with the worktree the response attributed to it, shows a run id alone when the response attributed none, and never presents a request-level failure as per-run outcomes.
entry_points: Task detail -> Fan out task runs -> Isolate each run in its own worktree
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-task-fanout-worktree-isolation
---

QA impact: Task 07 adds the fan-out isolation row, its count statement, and per-run result attribution.
