---
id: TA-worktree-web-fanout-isolation
area: TA
title: Isolate fan-out runs from the fan-out dialog
persona: Bruno
journey: J-isolated-task-loop-execution
expected: The fan-out dialog offers per-run isolation, off by default, and states how many worktrees the current assignment lines would create using the same count the request carries. After an isolated fan-out the dialog reports each run with the worktree the response attributed to it, shows a run id alone when the response attributed none, and never presents a request-level failure as per-run outcomes.
entry_points: S11 Task detail -> Fan out task runs -> Isolate each run in its own worktree
qa_status: pass
bug_ids: BUG-20260813-web-fanout-missing-intent-identity
fix_status: fixed
retest_status: pass
fix_commits: 207bc4a7
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/screenshots/task-fanout-isolation.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/task-fanout-two-runs.json
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: TA-task-fanout-worktree-isolation
---

QA impact: Task 07 adds the fan-out isolation row, its count statement, and per-run result attribution.
