---
id: RT-worktree-web-composer-binding-fork
area: RT
title: Read a session's environment and fork it into a worktree
persona: Ada
journey: J-worktree-management
expected: The composer states the session's actual binding: the workspace root, or the bound worktree with a lock. The binding is never editable in place. The fork affordance appears only when the daemon reports /worktree available, and its refusal reason is shown verbatim when it is not. Confirming the fork states three facts, creates exactly one new clean session in the target worktree, and leaves the original session and its uncommitted changes untouched.
entry_points: S8 Session composer environment chip and /worktree command; S9 fork confirmation; S16 session header binding chip
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-eng-138-composer-20260825-001640-399858-lab/qa-artifacts/qa/composer-root.png; /Users/pedronauck/dev/qa-labs/compozy-eng-138-composer-20260825-001640-399858-lab/qa-artifacts/qa/composer-root-tooltip.png; web/src/systems/session/components/__tests__/session-environment-chip.test.tsx; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-08-24-eng-138.md
overlaps: RT-session-worktree-fork; RT-session-worktree-lifecycle
---

QA impact: Task 07 adds the composer binding chip, the fork dialog, the session-header binding chip, and command availability rendering.

2026-08-24 behavior change: ENG-138 replaces the visible workspace path with an icon-only control and
moves the environment and fork action into the focusable tooltip; re-walk the composer affordance,
including the verbatim unavailable reason and the unchanged `/worktree` command path.

2026-08-25 targeted walk: the daemon-served composer rendered the icon-only workspace control, exposed
the target and fork action in its accessible name and hover tooltip, and opened/cancelled the existing
fork dialog without changing the binding. The unavailable-reason wording remains covered by the
canonical component suite; the `/worktree` action path is covered by the existing web suite.
