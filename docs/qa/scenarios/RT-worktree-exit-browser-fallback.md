---
id: RT-worktree-exit-browser-fallback
area: RT
title: Continue worktree exit without forge credentials
persona: Ada
journey: J-worktree-management
expected: With no usable forge provider or credential, local exit actions remain available and the completion action uses a sanitized GitHub, GitLab, or Bitbucket compare URL, falling back to the remote root for an unknown host.
entry_points: compozy worktree exit|pr -o json; GET .../exit; POST .../exit/actions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-pr-idempotency; RT-worktree-api-surface-parity
---

QA impact: Task 05 adds the zero-credential browser tier without exposing credentials or URL secrets.
