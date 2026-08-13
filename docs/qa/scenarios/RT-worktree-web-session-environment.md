---
id: RT-worktree-web-session-environment
area: RT
title: Choose the environment when creating a session
persona: Ada
journey: J-worktree-management
expected: Session create shows an environment control under Workspace with the workspace root selected and only ready worktrees offered. The control is absent entirely on a workspace git cannot back. Changing workspace resets the choice, a selection whose worktree stops being ready blocks Start with a named reason, and picking New worktree materializes through the worktree API with its real phases before the session is created bound to the ready id.
entry_points: S7 Session create dialog (Advanced) -> Environment
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-bound-context.png; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-web-create-adopt
---

QA impact: Task 07 adds the session-create environment control and the materialize-then-bind create path.
