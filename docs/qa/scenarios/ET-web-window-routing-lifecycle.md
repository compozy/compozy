---
id: ET-web-window-routing-lifecycle
area: ET
title: Operate window lifecycle with URL and semantic topology parity
persona: Bruno
journey: J-operate-desktop-shell
expected: Dock, palette, pointer, and keyboard activation open or focus one window instance; drag, structural resize, zoom, minimize, restore, and close preserve return anchors and successor focus; the focused window owns the URL with one history write per user cause; task/detail/search route intent survives reload and daemon restart, layout undo does not rewind it, and browser, CLI, native tool, and peer-browser changes converge by revision.
entry_points: web desktop dock and windows; browser history; compozy window; compozy__window_manager
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-06-route-reload-general.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-web-desktop-shell-lifecycle; ET-window-manager-public-parity; ET-window-manager-layout-gestures; ET-web-route-chrome-topbar
---

story: As a builder, I can arrange persistent desktops and trust every surface to observe one semantic topology while my focus remains client-local.

qa-impact: 2026-07-22 window-management hard cut replaced independent window documents with semantic commands, structural return anchors, durable route intent, and revisioned convergence; 2026-07-24 explicit `window.focus` and minimized-window restore now activate the window's desktop for the issuing client (dock activation follows a cross-desktop window instead of failing silently). Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 semantic app/instance identity, per-tab route stacks, and active-member routing
replaced singleton window lookup. Reset for tabbed routing and reload continuity.
