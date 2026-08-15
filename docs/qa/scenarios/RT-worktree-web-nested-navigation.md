---
id: RT-worktree-web-nested-navigation
area: RT
title: Navigate nested worktrees and scope work to one from the desktop shell
persona: Ada
journey: J-worktree-management
expected: The command switcher, menubar workspace menu, workspaces overview, and the shared nest row component render the same worktree nest from one query — same rows, locked order, full state vocabulary, adopted-only counts, discovered rows marked and selectable, and pending, missing, or error rows inert with their reason held in a capped trailing lane that never collides with the name, chip, or path. Every worktree of the workspace in view stays in the nest — no control folds rows behind an "All N worktrees" jump anywhere; past the host cap the rows region scrolls inside the shared ScrollArea (topbar submenu, switcher side panel, and the overview's vertical menu alike) while the hairline + New worktree footer stays pinned and reachable below the scroll. Hover or ArrowRight opens the side submenu on S1 and S2; a pointer click on a workspace selects it. Keyboard-only traversal reaches every nested entry — arrowing past the cap scrolls the focused row into view. Selecting a worktree scopes session and task reads server-side and the menubar chip reads `workspace / worktree`. A window opened after the pick inherits the selection — the chip never resets to the parent on window open or focus. A window that makes its own pick keeps it independently of later shell gestures; re-selecting the active workspace roots only the acting scope, leaving other windows' picks intact.
entry_points: S1 command switcher workspace row (hover/ArrowRight side submenu); S2 OS menubar workspace menu/chip; S3 Workspaces overview Worktrees control; S5 Worktree row/status chip
qa_status: blocked-verify
bug_ids: BUG-20260813-desktop-shell-context-order; BUG-20260813-pending-worktree-marked-missing
fix_status: fixed
retest_status: pass
fix_commits: 8ec45d75; b6eb94d0
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/screenshots/nested-worktree-selected.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-terminal-rewalk-20260813-150834-409343-lab/qa-artifacts/qa/worktree-list-ready.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-scroll-area-20260814-184457-553088-lab/qa-artifacts/qa/web-worktree-scroll-before.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-scroll-area-20260814-184457-553088-lab/qa-artifacts/qa/web-worktree-scroll-after.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-scroll-area-20260814-184457-553088-lab/qa-artifacts/qa/cli-list-worktrees.json
last_report: docs/qa/reports/2026-08-14-worktree-scroll-area.md
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

2026-08-14 behavior change: opening any window used to reset the chip to the parent workspace
because the fresh window scope started empty. Scopes without their own pick now inherit the shell
selection (per-scope map fallback), every selection gesture also sets the shell ambient default,
and a same-workspace re-selection roots only the acting scope instead of wiping every window's
pick. Covered by the store suite (active-workspace-store.test.ts) and the extended os-shell e2e
(select worktree -> open Tasks window -> chip persists).

2026-08-14 behavior change (overflow gate): the "All N worktrees" row now renders only when
adopted records overflow the fold (locked S1 gate: adopted > 5) — a nest overflowing on discovered
entries alone shows no overflow row, so "All 2 worktrees" can no longer appear beneath five
discovered rows. Covered by worktree-sort.test.ts (UT-134 pure half).

2026-08-14 behavior change (S2 side submenu): the menubar workspace menu no longer folds worktrees
inline beneath the workspace row. A git-backed workspace row is now a submenu trigger — hover (or
ArrowRight/Enter) opens a side submenu holding the same sorted and truncated worktree rows, the
adopted-only overflow row, and New worktree; a pointer click on the workspace row selects that
workspace (rooting the acting scope) and closes the menu, and every selection closes the menu
(native menu idiom). Nest rows (S1/S2/S3 shared component) are now two lines — state dot + name,
then the path with a small quiet state chip — and the Adopt label reveals on row hover/focus in
S2. Each S2 row carries a three-dot actions item opening Copy path and, for adopted non-missing
records, Remove… (the existing remove dialog). Covered by the workspace-menu unit suite,
worktrees.spec.ts and os-shell.spec.ts e2e, and the VC-15/VC-18 story captures.

2026-08-14 behavior change (S1/S3 side submenu): the command switcher and workspaces overview
drop the inline expand-in-place nest. Git-backed rows open the shared worktree submenu to the
side on hover or ArrowRight/focus; a pointer click on the workspace still selects it and closes
the picker. The overview card stays a card — its Worktrees control is the submenu trigger.
Covered by the workspace-command-select unit suite and the WorktreeNavSwitcher /
OsWorkspacesOverview story captures.

2026-08-14 focused rewalk (large catalog): an isolated workspace with 18 real linked worktrees
matched the structured CLI catalog. The S3 submenu used a 380px `ScrollArea` for 864px of content,
reached `scrollTop=484` without moving the document, and kept New worktree fixed and visible.
Pointer hover, ArrowRight entry, ArrowLeft return focus, and a full-page refresh all passed.

2026-08-15 behavior change (one nest, scroll everywhere — reset to untested): the truncation
path is deleted end to end. `truncateWorktreeNest` and the "All N worktrees" overflow rows are
gone from every host (topbar submenu, command switcher panel, workspaces overview); the topbar
overflow no longer jumps to the overview. All hosts render the shared `WorktreeNestRow` anatomy
(dot + name, then sm chip + path) inside the shared `WorktreeNest` scroll frame with the create
footer pinned below the scroll; the topbar submenu gains the same `ScrollArea` cap the panel
already had. Inert reasons now truncate in a capped mono lane (`--max-width-worktree-nest-reason`)
with the full text in the row description — a long git reason (e.g. "gitdir file points to
non-existent location") can no longer crush the name column and paint over the state chip. The
walk must confirm: >5-worktree nest shows every name via scroll on S1, S2, and the overview; no
overflow control anywhere; create row reachable after scrolling; inert reason readable and
truncated; scope selection still lands `workspace / worktree` on the chip.

2026-08-15 blocked-verify: the formal walk is reserved to the operator-invoked `/qa-execution`
(disable-model-invocation); recorded the block instead of a verdict. Pre-staged evidence: unit
suites green (workspace-menu scroll/no-overflow assertions, overview scroll-slot assertions,
sort suite without the truncation half — web lane 604 files / 4983 tests); visual-contract
bundles VC-01..03 under `.compozy/tasks/unify-worktree-nest-scroll/evidence/visual/`; live
operator confirmation of the overview nest listing adopted + all discovered rows against the
real daemon.
