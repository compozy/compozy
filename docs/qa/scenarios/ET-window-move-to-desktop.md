---
id: ET-window-move-to-desktop
area: ET
title: Send the focused window to another desktop by name
persona: Théo
journey: J-organize-tabbed-work
expected: Window › Move window to lists every other desktop by name (and nothing when there is only one desktop, with the item disabled and a reason); choosing one moves the focused window — or its whole tab deck — to that desktop in one layout change, the source desktop reflows, the active client stays on the source desktop, the destination's pager dot and overview thumbnail gain the window, and undo brings it back; the palette commands `window.move_to_desktop.1` … `.9` (unbound by default) move to the desktop in that position; the move survives reload.
entry_points: menubar Window › Move window to; command palette Move window to desktop N; compozy window move --desktop
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-desktop-pager-overview; ET-window-manager-public-parity; ET-window-tab-deck-lifecycle
---

Added 2026-09-30 for the shell rail (VC-11 move window to desktop).

Walk: with three desktops, move a tiled window and then a two-tab deck from desktop 1 to desktop 3 via the Window menu; confirm desktop 1 reflows, the client stays on desktop 1, the overview shows the window on desktop 3; undo; bind `window.move_to_desktop.2` in Settings › Layouts › Shortcuts and use it; reload and confirm placement; with a single desktop confirm the item is disabled with a reason.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
