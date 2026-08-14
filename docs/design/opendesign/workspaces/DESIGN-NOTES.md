# Workspaces overview — Command-Tab switcher

Problem: production `os-workspaces-overview.tsx` is already a full-screen dialog, but it
reads as a dossier grid — 264px cards (`--width-workspace-card`) with members, path
footers and an Enter row. The menubar chip still opens a left-edge popover. Neither
matches how people actually switch: glance, arrow, land.

Redesign: **the overview is a macOS Command-Tab overlay.** Full-screen scrim over the
live OS shell. Centered glass strip of compact tiles (icon well + name). The focused
tile's identity is written once under the strip. Cards are ~80px, not 264px.

Spaces overview (`⇧⌘S`, window arrangements + thumbnails) is a different surface and
stays in `os/`. This set is workspace *identity* switching.

## Locked decisions

- **Host.** `.overlay.wsov` over the existing shell (menubar + desktop + dock remain
  visible, `inert` while open). Not a left popover, not a WindowFrame, not a dialog
  card. Chip click opens this overlay — the left `.ws-pop` is not the overview.
- **Tile.** 52px monogram well + 11.5px name. No members stack, no Enter →, no
  accent card border. Current = check on the well (success, same as menu `.pi-check`).
  Focus = a frosted glass plate behind the whole tile (macOS Command-Tab): a
  translucent light gradient + `backdrop-filter` blur + top light edge — no flat
  fill, no border, no focus ring on the well. The plate is the sole focus
  indicator (roving tabindex keeps it on the focused tile). Never a gray name.
- **One row.** Tiles never wrap. Few items: the glass hugs. Many items: the strip
  grows to `min(1080px, viewport)` and the track scrolls horizontally. Edge fades
  (glass dissolve + a dark veil) mark overflow — no scrollbar. Arrows keep the
  focused tile clear of the fade; trackpad / wheel anywhere on the overlay, or a
  drag flick, scrubs the row. Snap is proximity, not a hard carousel page.
- **Caption.** One block under the strip for the focused tile: name ·
  `root_dir` micro mono. Identity only — no agent/session counts.
- **Primary action is switch.** Click / ↵ on a tile switches and closes. **New
  workspace** is a trailing dashed tile, not a solid header button. Empty state is
  the only place New workspace is `.btn--primary` (nothing to switch to).
- **Global scope** (from `global-workspace/DESIGN-NOTES.md`): no Home card; counts =
  project workspaces; while global is on, no tile shows Current; picking a tile turns
  global off and switches in one gesture. Chip identity while on: `~` + **Global**.
- **Keyboard.** `⇧⌘W` opens; `←` `→` move (wrap jumps instantly, no long scrub);
  `↵` / Space activate; `esc` / scrim click closes. Roving tabindex in the strip.
  Focus returns to the chip. Home / End jump to the ends. Typing jumps: listbox
  typeahead on workspace names (prefix match; repeating a letter cycles its
  matches) — no visible UI, caption crossfades on every focus change.
- **Worktree menu (vertical, always visible).** The focused **git-backed**
  workspace shows its worktrees as a quiet vertical menu hanging under the
  caption — always, with no disclosure gesture. Worktrees are subordinate: a
  340px glass panel of **two-line rows** — name on the first line, a small
  state chip + the worktree path (micro mono, truncated) on the second (S5
  full anatomy at menu scale; `.wt-dot` state shapes pinned to the name
  line). One trailing signal with the `worktree-signals.tsx` priority;
  functional reasons stay in the right lane, never hover-only. **Never a
  horizontal strip** — worktrees must not compete with the workspace tiles
  for visual weight. The slot is zero-height in the stage flow, so the strip
  never shifts as menus change size between workspaces; and the stage sits
  **above dead center** — a reserved band (`clamp(160px, 30vh, 300px)`)
  below it keeps room for the tallest menu plus breathing space before the
  bottom-pinned shortcut hints. Truncate at 5 + quiet
  `All N worktrees` row (adopted-only count → worktree overview); `New
  worktree` is the last row after a hairline. Git-backed + zero worktrees →
  **only the dashed New worktree button**, no empty panel. Non-git: nothing —
  absent, never disabled.
- **Row actions (kebab).** Every worktree row carries a `⋮` button, revealed
  on hover/focus/selection, opening a small glass popover: **Copy path**
  (clipboard + toast) and **Delete worktree…**. Delete is a prototype toast —
  the real flow is `DELETE /api/workspaces/:id/worktrees/:wt` with the S13
  remove dialogs (dirty/unpushed refuse without force). One popover at a
  time; esc / outside click / focus move closes it.
- **Menu keyboard.** `↓` moves focus from the strip into the first menu row;
  `↑` at the top row and `esc` return to the strip (esc closes the overlay
  only from the strip layer); `↑` `↓` move, inert rows (pending/missing) are
  skipped; `↵` scopes. Discovered rows are selectable — `↵` reaches the same
  Adopt confirm as S1/S3 (US-009/ADR-002); `All` / `New` are real cross-links
  into the `worktree/` set.
- **Worktree scope (US-011).** `↵` on a menu row scopes to
  `workspace / worktree` and closes; `↵` on a workspace tile always lands on
  the root. The check sits on the menu row — one selection marker per layer —
  and the parent tile carries a **branch badge** (same pill, `git-branch`
  glyph) while a worktree is the scope. The menubar chip reads
  `compozy / payments-retry`, trigger parity with S1.
- **Caption is identity only.** `name` + `root_dir`. The agents · sessions
  clause was dropped (and agents left the subtitle) — the menu already says
  what is alive via the agent dot; counts stay adopted-only in the subtitle.
- **Worktree projection parity.** Same query as S1/S2/S3 (`worktree/` set):
  enum `ready · pending · discovered · missing · error`, adopted-only counts
  (discovered listed, never counted), locked sort ready → pending → discovered
  → missing → error. The surfaces must not diverge.
- **Accent budget.** Happy path: none on the overlay. Empty: New workspace fill.
  Global-on: switch track only. Live dots on windows behind the scrim — and the
  agent-activity dot on a worktree tile — are runtime state, not decoration,
  and don't count.
- **Gating.** Tiles = `GET /api/workspaces` project items. Meta =
  `use-workspace-details` agent/session rollups. Worktrees =
  `GET /api/workspaces/:id/worktrees` → `{worktrees[], discovered[],
  repo.git_backed}` (production projection: `worktree-display.ts` /
  `worktree-sort.ts`; old-overview precedent: `os-workspaces-overview.tsx`
  meta line + Show worktrees expander + `WorktreeNestList`). New workspace =
  existing directory picker. No invented metrics.

## Content model (production, kept)

`name`, 2-letter monogram, `root_dir`, Current marker, New workspace, worktree
nest projection (name, state, one signal, reasons) kept as vertical rows.
Dropped from the 264px card: Members, Enter →, accent border, hover lift,
and the `agents · sessions` meta (menu + subtitle carry what matters).

Shared story with global-workspace, expanded for the many-row: **compozy** (current) ·
**branas-site** · **notes** · **agh-docs** · **payments** · **infra** · **design-system** ·
**sandbox** · **branas-ia** · **compozy-ext** · **site** · **cli**.

Worktree story (compozy): **payments-retry** (ready, claude running) ·
**auth-refresh** (ready, ↓2) · **fix-flaky-e2e** (ready, merged) ·
**docs-refresh** (pending) · **spike-sqlite-vacuum** (discovered, external path) ·
**hotfix-cors** (missing). payments ×2, infra ×1 (setup-failed flag) — 8 adopted
total. branas-site/agh-docs/sandbox/branas-ia/compozy-ext: git, zero worktrees.
notes/design-system/site/cli: not git-backed.

## Files

Final: `workspaces-overview.html` (interactive overlay on the OS shell; opens
worktree-scoped — chip `compozy / payments-retry`, branch badge on the tile).
Lab: `workspaces-overview-states.html` (empty, one, many, global-on, production
264px, worktree menu: canonical · non-ready vocabulary · absence).
`index.html` maps finals × labs. Worktree vocabulary source: `../worktree/` set.
