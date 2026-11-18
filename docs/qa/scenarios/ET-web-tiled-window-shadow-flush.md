---
id: ET-web-tiled-window-shadow-flush
area: ET
title: Windows in tabs and splits sit flush and flat
persona: Bruno
journey: J-operate-desktop-shell
expected: With the default zero gaps, a window alone in a tab fills the desk and windows in a split meet edge to edge as flat panes — no radius, border, or shadow — so the 1px seam hairline is the only divider and nothing bleeds against the rail or the desk edges; nested splits (columns, grid, main and stack) behave the same; setting `[window_manager.gaps]` back to 8 shows the desk in the gutters without any clipped shadow.
entry_points: web tab bar (tab context menu Split with ▸ and Arrange ▸); Window menu Arrange; [window_manager.gaps]
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-layout-gestures; ET-web-desktop-shell-lifecycle
---

Flagged 2026-08-20: zoomed Home left a hard-edged reddish smear at the bottom-left gutter because tiled chrome still painted `--shadow-window` (≈90px blur) into an 8–10px gap, then `os-desk` overflow and desktop `contain: strict` clipped it. Cast elevation now belongs only to floating frames.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). Tiling is gutterless by default (`gaps.*` = 0, SD-013 default change) and tiled frames are flat; `--shadow-window` is retired in favour of `shadow-elevated` on floating frames only. Walk the default and an explicit `gaps` of 8.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Floating windows, zoom and edge snapping are retired; a tab holds one window or a split tree, and every window is a flat pane. Title, expected and entry points rewritten (the elevated floating finish and the Zoom entry points no longer exist). Walk: one window alone in a tab; Split with ▸ from the tab context menu to make two panes; Arrange ▸ Grid with four windows; then `[window_manager.gaps]` inner/outer 8. Confirm no radius, border or shadow on any pane and no clipped shadow in the gutters, in light and dark. Reset for the current build.
