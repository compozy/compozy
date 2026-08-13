---
id: RT-worktree-web-exit-ladder
area: RT
title: Run the assisted exit from the worktree context
persona: Ada
journey: J-worktree-management
expected: The worktree context shows a five-field status strip that renders unknown facts as unknown, hides ahead/behind without a known upstream, and hides the PR cell with no forge. The exit control renders the daemon's plan verbatim: its primary position, its per-row blocked reasons, and a global pause that blocks every row with the same literal. A status read failure blocks the whole control and is stated in the strip.
entry_points: S6 Workspaces overview -> worktree row -> Context; S5 status chip
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-commit-scope; RT-worktree-api-surface-parity
---

QA impact: Task 07 adds the worktree detail context, status strip, and daemon-rendered exit control.
