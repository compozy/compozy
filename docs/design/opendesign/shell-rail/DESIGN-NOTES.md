# Shell — rail dock + gutterless tiling (proposal)

Board: `shell-rail.html` (append `?annotate` for numbered callouts + legend).
Reference: `../image.png` (flat rail + topbar + hairline panes).

## What it represents
A restyle of the desktop shell chrome. Same apps, same menubar content, same window model
(layout tree, tabs, arrange, zoom, minimize). What changes is placement and finish.

## Production components it modifies
`OsDock` / `DesktopDock` (bottom glass dock → left rail) · `OsMenuBar` (glass → flat, mark moves to the
rail's top cell) · `OsWindowFrame` + `OsWindowDeck` (no radius, border, or shadow; the deck carries the
window controls) · `OsSnapSeam` + `OsWinLayer` (innerGap 8 → 0; the seam hairline is the only divider).

## States (VC ids)
- VC-01 default: session window (3 tabs) | Tasks (single tab, no deck) / Terminal (2 tabs)
- VC-02 switch tab: the active tab fuses with the head; selection stays neutral
- VC-03 seam resize: pointer drag or arrow keys (Shift = 10%); the hairline turns accent; 220px floor per pane
- VC-04 arrange: Window › Main and stack · Columns · Grid · Balance sizes
- VC-05 zoom: the focused window fills the desktop; the others stay in the tree
- VC-06 minimize: the pane leaves the tree; the rail shows a hollow ring and a dimmed glyph; click restores it
- VC-07 open app from rail: focuses an existing instance, otherwise splits the focused pane along its long side

## Decisions / deltas vs DS chapter 02
- Dock order, separators and glyphs are unchanged (`use-desktop-dock.ts`, `os-dock-icons.tsx`).
- Traffic lights become quiet minimize/zoom/close icons on the deck (or the head when there is no deck). Round
  lights read as floating-window chrome and don't fit flush tiles.
- Focus without borders: blurred windows dim their identity and trail to .6 (existing rule S-blur).
- S13 (82px dock band) is retired. The rail owns 56px of width instead.
- Chrome sits on `--rail` and panes on `--canvas`. No glass. The radius waiver no longer applies to frames.

## Open questions
- Does floating mode survive, or is tiling the only mode?
- Should compact mode (<960px) keep the bottom tab bar or collapse the rail?

---

# v2 — light, reference design language, desktops (`shell-rail-v2.html`)

Pedro's feedback on v1: it didn't cover multiple desktops, it invented a "Connected" topbar item, the rail
had dividers, and it should follow the reference image's design language (light) even where that departs
from DESIGN.md.

## Changes
- **Light system from the reference:** white chrome and panes, grey desktop, zinc hairlines, near-black
  primary (New session, Allow once, send), blue for links and the seam hover, and orange only for needs-you.
  Uses Inter. These tokens are local to the board, **not** `tokens.css`.
- **Rail:** no group dividers. The order is still `use-desktop-dock.ts`.
- **Topbar:** only what `OsMenuBar` renders at rest (globe, workspace chip, Session/Go/Window/Help, bell,
  ⌘K, settings). `MenubarLayoutStatus` and `MenubarUpdateIndicator` are conditional in production, so they
  are absent here.
- **Desktops:** the bottom `DesktopPager` has no home once the dock leaves the bottom edge, so it moves into
  the topbar as a named segmented switcher (name, window count, attention dot for off-screen needs-you,
  `+` new desktop), followed by a grid button that opens the desktops overview (`DesktopsOverview`). The
  overview shows live layout thumbnails, a keycap per position, a New desktop card, and Done/Esc.
  Each desktop keeps its own windows, tabs, layout tree, focus and minimized set. Switching slides 4% and
  fades over 240ms (current spec). ⌃1–⌃9 switch desktops in the prototype; the production chord is
  daemon-configured (`desktop.switch`).
- **Window › Move window to** sends the focused window to another desktop.

## States (VC ids, added)
VC-08 switch desktop · VC-09 desktops overview · VC-10 new empty desktop (empty card + New session) ·
VC-11 move window to desktop.

## Open questions
- Pager overflow above 7 desktops: the production pager windows ±2 with `…` controls. Should the switcher
  collapse into the overview instead?
- Does light become a theme next to dark, or replace it? This changes `tokens.css` + DESIGN.md (SD-level).

## v2 revision — 2026-09-29 (Pedro's round 2)
- **Traffic lights:** the bar spans the full width. The first 84px (`.lights`) are reserved for the macOS
  window controls of the frameless window, and the mark follows them. The rail starts below the bar.
- **Desktops → dots:** the segmented switcher is replaced by production-style pager dots, centred in the
  bar's free space. The active dot is an 18px pill, and an orange dot marks an off-screen desktop that needs
  you. Clicking the active dot toggles the overview. New desktop lives in the overview.
- **Window tabs → browser tabs:** tabs sit in a recessed strip. The active tab is a surface plate with
  rounded top corners and concave feet, fused with the head. Inactive tabs are split by hairlines that hide
  next to hover and next to the active tab. Tabs are 208px wide and shrink to 112px.
- **Dark mode:** `[data-theme="dark"]` inverts the same zinc ramp (same grammar, no warm tint). The toggle
  is in the rail foot, persists in localStorage, and can be forced with `?theme=dark`. VC-12.
- **Rail foot:** the New session button is removed (New session is still available in the Session menu,
  the tab `+` and the empty-desktop card). Settings moved from the topbar to the rail foot, below the
  theme toggle.

## v2 revision — round 3
- **Desktops:** the pager dots moved into the tray, right before a new **All desktops** grid button next to
  the bell. The dots only select a desktop, and the grid button toggles the overview.
- **Tab feet fixed:** the concave feet are now a 10px border-radius box with a surface box-shadow, overlapping
  the tab edge by 1px, so the arc meets the tab's side border and the strip's bottom hairline with no seam.
  The earlier radial-gradient feet left a gap and a misaligned arc.

## v2 revision — round 4: design system from aicss.dev Approval Card
The v2 visual system is replaced by the reference's `--ab-*` tokens: exact values in dark, the source's own
light set in light. Details live in `brand-spec.md`. Dark is now the default (`?theme=light` or the rail
toggle switches it).
Applied to: the chrome (rail and topbar on `chrome`), panes on `surface`, the tab strip and tool lists on
`sunken`, pill buttons and filters, Inter 425/500 type scale, identity wells in root window heads and the
approval card, state glyphs (spinner / dashed ring / check / amber dot), the approval card anatomy
(well + title, sunken command, hint + Deny + inverted "Allow once ↵"), and `shadow-card` on the composer,
cards and the active rail item. The needs-you signal stays Compozy orange (#e8572a dark, #d14e25 light); the amber is kept only for real warnings (terminal).
