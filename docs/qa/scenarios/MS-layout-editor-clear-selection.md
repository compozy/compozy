---
id: MS-layout-editor-clear-selection
area: MS
title: Return the layout inspector from a window to its tab with pointer or keyboard
persona: Bruno
journey: J-administer-window-manager
expected: On Settings › Layouts › Project layout, each window on the canvas is a toggle button (pressed when it is open in the inspector); picking a tab in the Tabs strip, by click or by Enter/Space on the focused tab, clears the window selection and returns the inspector to the tab's view (Position, Windows, Pinned, Opens on this tab, with Move tab left, Move tab right, Pin/Unpin, Open desktop here and Close tab); after an edit that moves or removes the selected window the inspector follows the window to its new tab or falls back to the tab view instead of pointing at a window the draft no longer has; there is no canvas background button.
entry_points: Settings › Layouts › Project layout; Tabs strip; layout canvas windows; Selection inspector
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager; ET-layout-editor-split-weights
---

story: As a keyboard operator, I can leave a layout selection without needing a pointer.

qa-impact: 2026-07-28 replaced a click handler on the canvas container with a semantic background button and added keyboard activation coverage. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-11-13 window-tabs-split S10. The canvas is now the shell's split view filled
edge to edge with windows, so the "Clear layout selection" background button is gone; the Tabs
strip owns returning to the tab view, and selection survives edits but not deletions. Walk:
preconditions — a desktop with a split tab of two windows and a second tab. Open Settings › Layouts
and pick the split tab. 1) Click a window: it shows as pressed and the inspector title is that
window's name. 2) Click the same tab in the Tabs strip: no window is pressed and the inspector shows
the "Tab" section. 3) Select a window again, then Tab to the Tabs strip and press Enter on the split
tab: same result from the keyboard. 4) Select a window and press Move to new tab: the canvas switches
to the new tab with that window still selected. 5) Select a window of a split and press Remove from
layout: the inspector falls back to the tab view. Confirm no control named "Clear layout selection"
exists. Reset for the current build.
