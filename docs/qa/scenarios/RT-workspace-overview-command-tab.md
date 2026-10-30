---
id: RT-workspace-overview-command-tab
area: RT
title: Switch workspace identity through the Command-Tab overview
persona: Ada
journey: J-operate-workspace-context
expected: ⌘⇧O (Workspace picker) toggles a full-screen overlay over the inert live shell — a single-row strip of monogram tiles that never wraps, with edge fades and wheel/drag scrubbing on overflow. Arrows move a focus plate with modulo wrap; Home/End jump; typeahead prefix-jumps with no visible UI; the caption crossfades to the focused workspace's name and ~-contracted root_dir. A workspace with sessions working right now carries a mint dot on its well's top corner — on the overlay tiles, and after the name in the workspace chip menu — announced as "N sessions working" and cleared when its turns settle. The current workspace carries a success check on its well — a git-branch glyph while a worktree is scoped, when the check moves to the scoped menu row instead. ↵ or click on a tile always switches to the workspace root and closes; while Global is on no tile reads current, the notice line renders, and picking a tile turns Global off in one gesture. The focused git-backed workspace always shows its worktrees as a vertical menu (locked sort, adopted-only counts; every worktree stays listed and a long list scrolls inside the menu's cap with the hairline + New worktree footer pinned below the scroll; lone dashed button at zero; nothing at all for non-git). The stage anchors above dead center — the reserved band below it keeps the menu and breathing space clear of the bottom-pinned shortcut hints, and the strip never shifts as menus change size. ↓ enters the menu, ↑ at the top or esc returns to the strip, inert rows are skipped with their reasons readable in a capped lane (never overlapping the state chip), ↵ scopes workspace/worktree, arrowing past the cap scrolls the focused row into view, and each row's kebab (pointer or Shift+F10) offers Copy path and the gated Delete worktree…. Esc steps popover → menu → overlay, returning focus to the menubar chip. Zero project workspaces collapse the strip to the empty copy with the surface's only accent button.
entry_points: ⌘⇧O; Go menu → Workspace picker; workspace chip menu → Workspace picker; command palette → Workspace picker
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

qa-impact: 2026-09-30 expectation corrected against the product: the overlay is the Workspace picker on ⌘⇧O (Go menu, workspace chip menu, and palette carry the same chord); ⇧⌘W is not bound, and the shell-rail retirement of the glass tokens made the strip and focus plate solid. Expected text only; status fields untouched.

qa-impact: 2026-10-20 working-session dot: both switchers now mark each workspace
whose sessions are working, from the one cross-workspace session-facets aggregate
(`GET /api/sessions/facets?all_workspaces=true`, read only while a switcher is open
and re-read on catalog wakes). Verified on the dev server against the real daemon:
the two workspaces with running turns showed the dot in the chip menu and on the
overlay tiles; the dot coexists with the current check pill, and it cleared live
when one workspace's sessions settled. The walk must prove a background turn in a
non-current workspace lights its dot without switching, and that it clears once
that turn ends.

qa-impact: 2026-10-22 (#900 shell UI polish): the workspace chip menu rows dropped their
monogram wells, so the working dot follows the workspace name there; the overlay tiles keep
their monogram well and corner dot. Expected text only; status fields untouched.

## Project lifecycle regression

1. Register two projects in an isolated lab and retain stopped session history in one.
2. In the picker, open the project's context menu and choose Archive project. Confirm it.
3. Verify the project disappears from the picker, project cycle, sidebar and default All projects lists.
4. In Settings → General → Projects → Archived, unarchive it. Verify the same project ID, stopped
   history, worktrees and desktop layouts remain available.
5. With a non-stopped session, a live Loop or an enabled automation, try archiving. Verify
   `workspace_busy` and the named blockers in the dialog; no state changes.
6. Open Remove project. Verify the history/bindings/new-ID warning, files-on-disk guarantee,
   Cancel, and Archive instead. Confirm removal only for disposable lab data.
7. Repeat archive/list/unarchive through CLI, HTTP, UDS and native tools. Explicit new session
   admission while archived must fail with `workspace_archived`; history reads must succeed.

Pinning and recent-use ordering are outside this regression.

### Project context menu on every project surface (issue #1029)

Repeat step 2's Archive project / Remove project entry from each surface where a project
appears; each opens the same actions and confirm dialog, and none shows the browser's native
Copy menu:

- the overview tile (right-click, or focus the tile and press Shift+F10);
- a project group header in the session list and Sessions modal under All projects with Group
  by project (right-click, or Shift+F10 on the group toggle); the User and profile group
  headers have no project menu;
- a project swimlane header on the Sessions board grouped by project;
- a project row in the menubar workspace menu (right-click or Shift+F10): the workspace menu
  closes and the project menu opens at the pointer; Archive project keeps its confirm open
  after the menu closes. Dismiss the project menu with Escape before choosing an action:
  focus must return to the project menubar trigger, even though the source row has unmounted.

### Archive project from the menubar project submenu (issue #1057)

1. Open the menubar workspace menu and hover (or ArrowRight onto) a git-backed project row.
   The submenu ends with a separator and **Archive project**, below New worktree. It never
   offers Remove project; a non-git project row has no submenu and keeps only its right-click
   menu.
2. Choose Archive project. The workspace menu closes and the same Archive confirm the overview
   uses opens. Cancel: focus returns to the project menubar trigger.
3. Repeat with a project that has a non-stopped session (for example the current one). Confirm:
   the dialog shows `workspace_busy` with the named blockers, and nothing changes.
4. Repeat with an idle project and confirm. It disappears from the menubar project list.

qa-impact: 2026-10-28 archive/remove lifecycle slice passed against an isolated daemon and
production web bundle. See `docs/qa/reports/2026-10-28-archive-projects.md`; older unrelated
scenario verdicts above are retained rather than inferred from this focused walk.

qa-impact: 2026-10-29 merge integration: context-menu composition must not change the
strip's tile population or scroll-settle focus. At 390px width with two projects,
hover the second project's tile, drag the strip slightly, then select worktrees.
Focus must remain on the geometrically nearest project and its worktree menu must
remain actionable. The existing mixed-selection overview E2E owns this regression.

Focused verification: the real-browser mobile drag/removal case failed on the old
bundle and passed twice on the corrected production bundle (2026-10-29).

## Overlay focus handoff regression (#1035)

1. Open Projects with Cmd+Shift+O. Use End and Enter to open New project.
   The incoming dialog owns focus immediately; the hidden menubar trigger must
   not receive focus during the handoff. Cancel and reopen Projects.
2. Focus a project tile and open its context menu. Focus must enter the menu before
   the overview is hidden from assistive technology. Escape restores the tile.
3. Open Archive project from that menu. Focus must enter the confirmation before
   its originating surface becomes hidden. Cancel without archiving the project.
4. Close Projects with Escape. Focus returns to the project menubar trigger.
5. Observe focus transitions and the browser console throughout. No focused
   descendant may belong to an aria-hidden/inert ancestor; modal isolation remains.

Focused evidence and limits: [2026-10-29 overlay focus report](../reports/2026-10-29-overlay-focus-handoff.md).
This focused regression does not supersede unrelated scenario verdicts above.
