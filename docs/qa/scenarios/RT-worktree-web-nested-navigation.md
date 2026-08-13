---
id: RT-worktree-web-nested-navigation
area: RT
title: Navigate nested worktrees and scope work to one from the desktop shell
persona: Ada
journey: J-worktree-management
expected: The command switcher, menubar workspace menu, workspaces overview, and shared row/status component render the same nested worktree tree from one query — same rows, locked order, full state vocabulary, adopted-only counts, discovered rows marked and selectable, and pending, missing, or error rows inert with their reason. Keyboard-only traversal reaches nested entries. Selecting a worktree scopes session and task reads server-side, the menubar chip reads `workspace / worktree`, and two open windows hold independent selections.
entry_points: S1 command switcher "Worktrees" group; S2 OS menubar workspace menu/chip; S3 Workspaces overview; S5 Worktree row/status chip
qa_status: pass
bug_ids: BUG-20260813-desktop-shell-context-order; BUG-20260813-pending-worktree-marked-missing
fix_status: fixed
retest_status: pass
fix_commits: 8ec45d75; b6eb94d0
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/screenshots/nested-worktree-selected.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/worktree-list-ready.json
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-web-create-adopt
---

QA impact: Task 06 adds nested worktree navigation, per-window worktree selection (store v3), and
server-side worktree scoping for session and task lists. The Phase C walk must confirm all three
surfaces agree, that a non-git workspace shows no worktree affordance at all (absent, not disabled),
that a git-backed workspace with zero worktrees shows no group noise, that nests past five entries
truncate behind an adopted-only "All N worktrees" row, and that a selection whose worktree goes
missing falls back to the parent workspace with the notice while the list stops filtering.

2026-08-13 fix replay: the desktop mounts successfully below its shell provider. The workspace menu
showed the ready checkout and the older intentionally missing record with distinct truthful states;
a new accepted creation moved from pending to ready through the live catalog stream. The complete
cross-surface and independent-window matrix continues in the Task 10 charter.
