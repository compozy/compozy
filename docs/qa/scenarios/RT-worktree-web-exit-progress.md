---
id: RT-worktree-web-exit-progress
area: RT
title: Follow an exit action through its announced phases
persona: Ada
journey: J-worktree-management
expected: Starting an exit action announces every phase up front, advances each phase from the per-worktree stream, states a skipped phase's reason in the daemon's words, attributes a mid-chain failure to its own phase with the captured command output, and ends with the single follow-up action the daemon named.
entry_points: S14 Worktree context -> exit action -> progress
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-push-publish
---

QA impact: Task 07 adds streamed exit progress driven by per-worktree events.
