---
id: RT-worktree-exit-merged-cleanup
area: RT
title: Clean up a merged worktree safely
persona: Ada
journey: J-worktree-management
expected: Assisted exit gives fresh forge-merged evidence precedence, distinguishes local-only from remote cleanup, blocks on stale or conflicting evidence, and never removes a worktree while an exit operation is running.
entry_points: compozy worktree status --forge; compozy worktree exit|remove -o json; GET .../exit; POST .../exit/actions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-pr-idempotency; RT-worktree-cli-lifecycle
---

QA impact: Task 05 adds merged-evidence precedence, two-tier cleanup guidance, and the atomic removal fence.
