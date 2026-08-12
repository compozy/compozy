---
id: RT-worktree-web-nested-navigation
area: RT
title: Navigate nested worktrees and scope work to one from the desktop shell
persona: Ada
journey: J-worktree-management
expected: The command switcher, the menubar workspace menu, and the workspaces overview render the same nested worktree tree from one query — same rows, same locked order, adopted-only counts, discovered rows marked and selectable, pending and missing rows inert with their reason. Keyboard-only traversal reaches nested entries. Selecting a worktree scopes session and task reads server-side, the menubar chip reads `workspace / worktree`, and two open windows hold independent selections.
entry_points: OS menubar workspace chip; ⌘K palette "Worktrees" group; Workspaces overview
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-web-create-adopt
---

QA impact: Task 06 adds nested worktree navigation, per-window worktree selection (store v3), and
server-side worktree scoping for session and task lists. The Phase C walk must confirm all three
surfaces agree, that a non-git workspace shows no worktree affordance at all (absent, not disabled),
that a git-backed workspace with zero worktrees shows no group noise, that nests past five entries
truncate behind an adopted-only "All N worktrees" row, and that a selection whose worktree goes
missing falls back to the parent workspace with the notice while the list stops filtering.
