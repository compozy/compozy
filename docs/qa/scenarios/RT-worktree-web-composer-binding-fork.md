---
id: RT-worktree-web-composer-binding-fork
area: RT
title: Read a session's environment and fork it into a worktree
persona: Ada
journey: J-worktree-management
expected: The composer states the session's actual binding: the workspace root, or the bound worktree with a lock. The binding is never editable in place. The fork affordance appears only when the daemon reports /worktree available, and its refusal reason is shown verbatim when it is not. Confirming the fork states three facts, creates exactly one new clean session in the target worktree, and leaves the original session and its uncommitted changes untouched.
entry_points: Session composer environment chip; /worktree command; session header binding chip
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-worktree-fork; RT-session-worktree-lifecycle
---

QA impact: Task 07 adds the composer binding chip, the fork dialog, the session-header binding chip, and command availability rendering.
