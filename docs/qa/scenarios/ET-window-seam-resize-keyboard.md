---
id: ET-window-seam-resize-keyboard
area: ET
title: Resize tiled panes by dragging or keying the shared seam
persona: Bruno
journey: J-administer-window-manager
expected: Between tiled panes the only divider is a 1px hairline with a 9px hit area; hovering, focusing, or dragging it turns it into a 2px accent line and the resize cursor; dragging resizes the panes on both sides live and commits one layout change; a focused seam (a separator named "Resize boundary N") moves 2% per arrow key and 10% with Shift; neither pane shrinks below 220px; the result survives reload and layout undo/redo restores it; the seam reads the same in light and dark.
entry_points: web desk seam pointer drag; Tab focus + arrow keys on a seam; Undo layout / Redo layout
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-window-manager-layout-gestures; ET-web-tiled-window-shadow-flush; ET-window-arrange-presets
---

Added 2026-09-30 for the shell rail (VC-03 seam resize). Tiling is gutterless by default, so the seam is the only resize handle between panes.

Walk: tile two windows side by side, hover the seam (2px accent), drag it to roughly 70/30, reload; Tab to the seam and press → five times (2% each) then Shift+→ once (10%); try to push a pane below 220px (it stops); undo and redo the layout; repeat on a horizontal seam in a stacked column; switch the theme and confirm the hairline and the accent state stay visible.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
