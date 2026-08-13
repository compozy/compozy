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
  `{n} agent(s) · {m} session(s)` (zero counts omitted) · `root_dir` micro mono.
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
- **Accent budget.** Happy path: none on the overlay. Empty: New workspace fill.
  Global-on: switch track only. Live dots on windows behind the scrim don't count.
- **Gating.** Tiles = `GET /api/workspaces` project items. Meta =
  `use-workspace-details` agent/session rollups. New workspace = existing directory
  picker. No invented metrics.

## Content model (production, kept)

`name`, 2-letter monogram, `agents · sessions`, `root_dir`, Current marker, New
workspace. Dropped from the 264px card: Members, Enter →, accent border, hover lift.

Shared story with global-workspace, expanded for the many-row: **compozy** (current) ·
**branas-site** · **notes** · **agh-docs** · **payments** · **infra** · **design-system** ·
**sandbox** · **branas-ia** · **compozy-ext** · **site** · **cli**.

## Files

Final: `workspaces-overview.html` (interactive overlay on the OS shell).
Lab: `workspaces-overview-states.html` (empty, one, many, global-on, production 264px).
`index.html` maps finals × labs.
