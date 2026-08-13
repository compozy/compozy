---
id: RT-worktree-web-merged-cleanup
area: RT
title: Read cleanup evidence before removing a finished worktree
persona: Ada
journey: J-worktree-management
expected: Cleanup evidence states its source and verdict: a forge-merged request, local safe-to-clean evidence, or a downgraded verdict in an informational tone when the branch still exists on the remote. A blocker suppresses the Clean up action entirely rather than disabling it, and a merged indicator flips rather than disappearing when a request closes without merging.
entry_points: S14 Worktree context -> cleanup evidence row
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-merged-cleanup
---

QA impact: Task 07 adds the merged/cleanup evidence row and its suppression rule.
