---
id: RT-worktree-reconcile-branch-safety
area: RT
title: Reconcile missing worktrees and reclaim only unchanged run branches
persona: Théo
journey: J-worktree-management
expected: Removing a checkout outside Compozy creates a missing tombstone without deleting sessions, runs, events, or branches; restore accepts only the recorded Git identity, normal removal preserves branches, and automatic reclamation deletes only a Compozy-created run-namespace branch whose ref still equals its recorded head.
entry_points: compozy worktree list --refresh|adopt|inspect|remove|dismiss -o json; HTTP/UDS list|adopt|inspect|remove|dismiss routes; worktree.missing|removed|branch_reclaimed events; Workspaces overview Resolve
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.json; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-web-missing-resolution; RT-worktree-web-removal-two-step; RT-worktree-exit-merged-cleanup
---

QA impact: Tasks 01, 02, and 06 expose reconcile, removal, and recovery. The walk must replace the
old path with another repository, move both eligible and changed run branches, and prove no
workspace prune, cascade, unsafe leftover deletion, or compare-and-delete race can lose history.
