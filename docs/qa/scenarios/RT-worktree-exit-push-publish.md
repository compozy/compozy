---
id: RT-worktree-exit-push-publish
area: RT
title: Publish an assisted worktree branch
persona: Ada
journey: J-worktree-management
expected: Assisted exit advances from commit to push, publishes a branch without an upstream using tracked upstream setup, preserves an existing upstream, and reports one truthful completion action.
entry_points: compozy worktree exit|push -o json; GET .../exit; POST .../exit/actions; .../stream
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-commit-scope; RT-worktree-api-surface-parity
---

QA impact: Task 05 adds durable commit-and-push and push-only exit paths with progress replay.
