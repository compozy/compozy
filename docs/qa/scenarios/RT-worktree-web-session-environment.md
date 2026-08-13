---
id: RT-worktree-web-session-environment
area: RT
title: Choose the environment when creating a session
persona: Ada
journey: J-worktree-management
expected: Session create shows an environment control under Workspace with the workspace root selected and only ready worktrees offered. The control is absent entirely on a workspace git cannot back. Changing workspace resets the choice, a selection whose worktree stops being ready blocks Start with a named reason, and picking New worktree materializes through the worktree API with its real phases before the session is created bound to the ready id.
entry_points: S7 Session create dialog (Advanced) -> Environment
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-web-create-adopt
---

QA impact: Task 07 adds the session-create environment control and the materialize-then-bind create path.
