---
id: RT-workspace-overview-command-tab
area: RT
title: Switch workspace identity through the Command-Tab overview
persona: Ada
journey: J-operate-workspace-context
expected: ⇧⌘W toggles a full-screen overlay over the inert live shell — a single-row glass strip of monogram tiles that never wraps, with edge fades and wheel/drag scrubbing on overflow. Arrows move a frosted focus plate with modulo wrap; Home/End jump; typeahead prefix-jumps with no visible UI; the caption crossfades to the focused workspace's name and ~-contracted root_dir. The current workspace carries a success check on its well — a git-branch glyph while a worktree is scoped, when the check moves to the scoped menu row instead. ↵ or click on a tile always switches to the workspace root and closes; while Global is on no tile reads current, the notice line renders, and picking a tile turns Global off in one gesture. The focused git-backed workspace always shows its worktrees as a vertical menu (locked sort, adopted-only counts; every worktree stays listed and a long list scrolls inside the menu's cap with the hairline + New worktree footer pinned below the scroll; lone dashed button at zero; nothing at all for non-git). The stage anchors above dead center — the reserved band below it keeps the menu and breathing space clear of the bottom-pinned shortcut hints, and the strip never shifts as menus change size. ↓ enters the menu, ↑ at the top or esc returns to the strip, inert rows are skipped with their reasons readable in a capped lane (never overlapping the state chip), ↵ scopes workspace/worktree, arrowing past the cap scrolls the focused row into view, and each row's kebab (pointer or Shift+F10) offers Copy path and the gated Delete worktree…. Esc steps popover → menu → overlay, returning focus to the menubar chip. Zero project workspaces collapse the strip to the empty copy with the surface's only accent button.
entry_points: ⇧⌘W; Go menu → Workspaces…; S1 Workspace menu → Workspaces overview…
qa_status: blocked-verify
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

2026-08-15 layout + nest fixes (reset to untested): the reserved band below the
stage now actually renders — the token namespace was dead (`--basis-*` is not a
Tailwind v4 namespace; now `--flex-basis-workspaces-reserved`), so the stage was
dead-centered and a six-row menu sat on top of the shortcut hints. The menu also
adopts the unified scroll contract: every worktree listed, rows region capped by
`--height-workspaces-menu-max` (derived from the band) with internal scroll and
the New worktree footer pinned; there is no "All N worktrees" row. Inert reasons
truncate in a capped lane (full text in the row description) — the "gitdir file
points to non-existent location" overlap over the discovered chip is fixed. The
walk must verify: strip anchored above dead center with clear space between the
tallest menu and the hints; a >6-row nest scrolling with ↓ traversal reaching
every row; the broken-checkout reason readable without overlap.

2026-08-15 blocked-verify: the formal walk is reserved to the operator-invoked
`/qa-execution` (disable-model-invocation), so the agent recorded the block
instead of a verdict. Pre-staged evidence: visual-contract bundles VC-01..03
(`.compozy/tasks/unify-worktree-nest-scroll/evidence/visual/`, PASS, 0 blocking
divergences — anchoring parity, capped scroll, reason lane); live operator
confirmation on the dev server against the real daemon (strip above center,
all six compozy rows listed incl. five discovered, reason lane clean).
Observation to watch during the walk: discovered rows transiently vanished
from both operator and clean-session views (~13:56–14:00) while
`GET /api/workspaces/:id/worktrees` kept returning all five — self-resolved by
14:05 with no frontend change; suspect daemon discovery-snapshot timing after
the 13:03 adoption, not the UI. If it recurs, file a daemon-side bug.
