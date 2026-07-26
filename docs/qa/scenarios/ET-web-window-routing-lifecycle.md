---
id: ET-web-window-routing-lifecycle
area: ET
title: Operate window lifecycle with URL and semantic topology parity
persona: Bruno
journey:
expected: Dock, palette, pointer, and keyboard activation open or focus one window instance; drag, structural resize, zoom, minimize, restore, and close preserve return anchors and successor focus; the focused window owns the URL with one history write per user cause; task/detail/search route intent survives reload and daemon restart, layout undo does not rewind it, and browser, CLI, native tool, and peer-browser changes converge by revision.
entry_points: web desktop dock and windows; browser history; agh window; agh__window_manager
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-desktop-shell-lifecycle; ET-window-manager-public-parity; ET-window-manager-layout-gestures; ET-web-route-chrome-topbar
---

story: As a builder, I can arrange persistent desktops and trust every surface to observe one semantic topology while my focus remains client-local.

qa-impact: 2026-07-22 window-management hard cut replaced independent window documents with semantic commands, structural return anchors, durable route intent, and revisioned convergence; 2026-07-24 explicit `window.focus` and minimized-window restore now activate the window's desktop for the issuing client (dock activation follows a cross-desktop window instead of failing silently). Flag only; the next QA cycle owns live retesting.
