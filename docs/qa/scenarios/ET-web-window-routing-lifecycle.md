---
id: ET-web-window-routing-lifecycle
area: ET
title: Operate window lifecycle with URL and semantic topology parity
persona: Bruno
journey: J-operate-desktop-shell
expected: Dock, palette, pointer, and keyboard activation open or focus one window instance; tab drag and reorder, split resize, Move to new tab, Move to desktop, and close preserve return anchors and successor focus; the focused window owns the URL with one history write per user cause; task/detail/search route intent survives reload and daemon restart, layout undo does not rewind it, and browser, CLI, native tool, and peer-browser changes converge by revision.
entry_points: web desktop dock and windows; browser history; compozy window; compozy__window_manager
qa_status: untested
bug_ids: BUG-20260830-terminal-retarget-duplicate-window; BUG-20260902-background-window-stream-starvation
fix_status: fixed
retest_status:
fix_commits: pending-remediation-batch
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/theo-routing4-home-restored.png; docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-desktop-shell-lifecycle; ET-window-manager-public-parity; ET-window-manager-layout-gestures; ET-web-route-chrome-topbar
---

Open Tasks from the dock and navigate to a task detail. Race a palette activation with restoration of that deep link and verify one window, one route owner, and one history entry. Drag its tab to a new position, split it with another app and resize the split, use Move to new tab, close and Reopen closed tab, undo layout, reload, and restart the daemon; task route intent and focus must survive each supported transition.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-11-13 window-tabs-split S10. Zoom, minimize and restore are retired; a window only opens or closes, and moves between tabs, splits and desktops. Expected and the walk rewritten to the tab/split transitions (tab drag, split resize, Move to new tab, Move to desktop, close and Reopen closed tab). Walk the paragraph above with the task-detail deep link; confirm one window, one route owner and one history entry after each transition. Reset for the current build.

qa-impact: 2026-11-13 #1655. Open-or-focus (dock, palette, Go menu) writes one entry even when the stream shows the new focus first. Walk: Tasks focused → dock Files with no Files window, Back once → Tasks; dock Files again (window open), Back once → Tasks; repeat from the palette and the Go menu. The keyboard follows a focus change you make: two windows in a split, caret in the left composer, focus-right chord, type → the text lands in the right window; chord back → the caret is in the left composer again; Back to an earlier window's entry → that window holds the keyboard. Kept behavior: after a project switch, Back opens the entry's page in the current project (the URL names no project).
