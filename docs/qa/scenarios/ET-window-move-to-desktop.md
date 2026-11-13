---
id: ET-window-move-to-desktop
area: ET
title: Send a tab to another desktop by name
persona: Théo
journey: J-organize-tabbed-work
expected: Window › Move to desktop ▸ and a tab's Move to desktop ▸ list every other desktop by name (with one desktop the Window menu item is disabled with the reason "needs another desktop"); choosing one moves the whole tab, splits included, to the end of that desktop's tab bar in one layout change; the client stays on the source desktop, whose bar loses the tab (a desktop left with no tabs shows a New Tab window); `window.move_to_desktop.1–9` shortcuts and `compozy window move --desktop <id> --group` do the same.
entry_points: Window › Move to desktop; tab context menu › Move to desktop; window.move_to_desktop.1–9; compozy window move --group; compozy__window_move move_group
qa_status: untested
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

qa-impact: 2026-11-13 window-tabs-split S10. The destination is now the target desktop's tab bar; decks no longer exist. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. With one desktop, open Window: Move to desktop is disabled with "needs another desktop". Create desktops "Build" and "Review". On desktop 1 split two windows in one tab, then Window › Move to desktop › Review: the tab leaves desktop 1, the client stays there, and switching to Review shows the tab at the end of its bar with both windows still split. Move the last tab of a desktop away: that desktop shows a New Tab window. ⌘Z undoes the move. CLI: `compozy window move --id <w> --desktop <build-id> --group --revision <r>` moves the whole tab.
