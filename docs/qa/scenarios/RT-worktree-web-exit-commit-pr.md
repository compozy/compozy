---
id: RT-worktree-web-exit-commit-pr
area: RT
title: Commit and open a request through the exit dialogs
persona: Ada
journey: J-worktree-management
expected: The commit dialog shows the scope it will stage with counts and untracked additions listed by name, states when that list was bounded, and offers an honest default-message placeholder with no generation promise. Nothing-to-commit replaces the scope with the daemon's reason. The PR dialog resolves its base from the plan, carries only daemon-supplied starting text, places Cancel and a SplitButton in the ruled footer (draft is a menu alternative of create), collapses to a single view action for an already-open request, and shows the browser action alone with zero credentials.
entry_points: S14 Worktree context -> Commit / Open pull request
qa_status: skipped
bug_ids: BUG-20260813-worktree-exit-menu-crash
fix_status: fixed
retest_status: pass
fix_commits: d7869a8
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/screenshots/worktree-commit-scope.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/screenshots/worktree-exit-menu-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/api-worktree-exit-plan.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-worktree-exit-commit-scope; RT-worktree-exit-pr-idempotency; RT-worktree-exit-browser-fallback
---

QA impact: Task 07 adds the commit and pull-request dialogs, including the zero-credential absence contract.

2026-08-14 layout: the PR dialog uses the unframed Entity shell, FieldGroup spacing, and the
same ruled footer as commit (`SplitButton`, not body-width rows). Prefill, draft-as-menu
alternative, and zero-credential absence are unchanged.

Visible count is porcelain `changed_files`. Untracked additions are listed by name and are not added on top of that total.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
