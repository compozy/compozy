---
id: RT-worktree-web-merged-cleanup
area: RT
title: Read cleanup evidence before removing a finished worktree
persona: Ada
journey: J-worktree-management
expected: Cleanup evidence states its source and verdict: a forge-merged request, local safe-to-clean evidence, or a downgraded verdict in an informational tone when the branch still exists on the remote. A blocker suppresses the Clean up action entirely rather than disabling it, and a merged indicator flips rather than disappearing when a request closes without merging.
entry_points: S14 Worktree context -> cleanup evidence row
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/e2e/__tests__/worktrees.spec.ts; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-exit-merged-cleanup
---

QA impact: Task 07 adds the merged/cleanup evidence row and its suppression rule.
