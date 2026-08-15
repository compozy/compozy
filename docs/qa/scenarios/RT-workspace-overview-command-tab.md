---
id: RT-workspace-overview-command-tab
area: RT
title: Switch workspace identity through the Command-Tab overview
persona: Ada
journey: J-operate-workspace-context
expected: ⇧⌘W toggles a full-screen overlay over the inert live shell — a single-row glass strip of monogram tiles that never wraps, with edge fades and wheel/drag scrubbing on overflow. Arrows move a frosted focus plate with modulo wrap; Home/End jump; typeahead prefix-jumps with no visible UI; the caption crossfades to the focused workspace's name and ~-contracted root_dir. The current workspace carries a success check on its well — a git-branch glyph while a worktree is scoped, when the check moves to the scoped menu row instead. ↵ or click on a tile always switches to the workspace root and closes; while Global is on no tile reads current, the notice line renders, and picking a tile turns Global off in one gesture. The focused git-backed workspace always shows its worktrees as a vertical menu (locked sort, adopted-only counts, truncation at five plus a New worktree footer row; lone dashed button at zero; nothing at all for non-git). ↓ enters the menu, ↑ at the top or esc returns to the strip, inert rows are skipped with their reasons visible, ↵ scopes workspace/worktree, and each row's kebab (pointer or Shift+F10) offers Copy path and the gated Delete worktree…. Esc steps popover → menu → overlay, returning focus to the menubar chip. Zero project workspaces collapse the strip to the empty copy with the surface's only accent button.
entry_points: ⇧⌘W; Go menu → Workspaces…; S1 Workspace menu → Workspaces overview…
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/e2e/__tests__/os-shell.spec.ts; web/src/systems/os/components/__tests__/os-workspaces-overview.test.tsx; .compozy/tasks/workspaces-overview-redesign/evidence/visual/vc-wsov/; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/workspaces-current-worktree.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/workspaces-empty-state.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/keyboard-shortcuts-workspaces.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/settings-workspaces-shortcut.png
last_report: docs/qa/reports/2026-08-15-pr-410-review-evidence.md
overlaps: RT-worktree-web-create-adopt; RT-worktree-web-removal-two-step; MS-web-workspace-lists-hide-home
---

QA impact: the 264px dossier grid was hard-cut for the Command-Tab switcher
(`docs/design/opendesign/workspaces/` visual contract). Switching semantics, the
active-workspace store, and the three-surface worktree projection parity
(menubar menu · palette · overview) are unchanged; the overview's interaction
model, its ⇧⌘W shortcut, and the always-visible vertical worktree menu are new.
The walk must prove the keyboard model end-to-end, the one-marker-per-layer
current rule (tile check ↔ branch badge + row check), Global-on semantics, and
that overlay Delete routes through the existing force-refusal remove dialogs.
