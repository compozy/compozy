---
id: ET-web-tiled-window-shadow-flush
area: ET
title: Tiled and zoomed windows sit flush and flat; only floating windows lift
persona: Bruno
journey: J-operate-desktop-shell
expected: With the default zero gaps, tiled and zoomed windows meet edge to edge as flat panes — no radius, border, or shadow — so the 1px seam hairline is the only divider and nothing bleeds against the rail or the desk edges; split and edge-snapped panes behave the same; unzooming into a floating window or opening a floating window restores the floating finish (1px hairline + elevated shadow); setting `[window_manager.gaps]` back to 8 shows the desk in the gutters without any clipped shadow.
entry_points: web desktop window Zoom control; Window menu Zoom; command palette Zoom window; [window_manager.gaps]
qa_status: skipped
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
