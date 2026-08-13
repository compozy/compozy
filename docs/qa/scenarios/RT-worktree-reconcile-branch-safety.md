---
id: RT-worktree-reconcile-branch-safety
area: RT
title: Reconcile missing worktrees and reclaim only unchanged run branches
persona: Théo
journey: J-worktree-management
expected: Removing a checkout outside Compozy creates a missing tombstone without deleting sessions, runs, events, or branches; restore accepts only the recorded Git identity, normal removal preserves branches, and automatic reclamation deletes only a Compozy-created run-namespace branch whose ref still equals its recorded head.
entry_points: compozy worktree list --refresh|adopt|inspect|remove|dismiss -o json; HTTP/UDS list|adopt|inspect|remove|dismiss routes; worktree.missing|removed|branch_reclaimed events; Workspaces overview Resolve
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-web-missing-resolution; RT-worktree-web-removal-two-step; RT-worktree-exit-merged-cleanup
---

QA impact: Tasks 01, 02, and 06 expose reconcile, removal, and recovery. The walk must replace the
old path with another repository, move both eligible and changed run branches, and prove no
workspace prune, cascade, unsafe leftover deletion, or compare-and-delete race can lose history.
