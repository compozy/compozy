---
id: RT-worktree-web-exit-commit-pr
area: RT
title: Commit and open a request through the exit dialogs
persona: Ada
journey: J-worktree-management
expected: The commit dialog shows the scope it will stage with counts and untracked additions listed by name, states when that list was bounded, and offers an honest default-message placeholder with no generation promise. Nothing-to-commit replaces the scope with the daemon's reason. The PR dialog resolves its base from the plan, carries only daemon-supplied starting text, offers draft as a peer row, collapses to a single view row for an already-open request, and shows the browser row alone with zero credentials.
entry_points: Worktree context -> Commit / Open pull request
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-exit-commit-scope; RT-worktree-exit-pr-idempotency; RT-worktree-exit-browser-fallback
---

QA impact: Task 07 adds the commit and pull-request dialogs, including the zero-credential absence contract.
