---
id: RT-worktree-exit-commit-scope
area: RT
title: Commit the complete visible worktree change set
persona: Ada
journey: J-worktree-management
expected: The assisted commit preview names untracked files, excludes ignored files, reports the bounded change summary, and commits exactly the full visible worktree state with the supplied or deterministic default message.
entry_points: compozy worktree exit|commit -o json; GET .../exit; POST .../exit/actions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-cli-lifecycle; RT-worktree-api-surface-parity
---

QA impact: Task 05 adds the named-untracked stage-all commit contract and streamed step results.
