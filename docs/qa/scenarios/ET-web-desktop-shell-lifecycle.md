---
id: ET-web-desktop-shell-lifecycle
area: ET
title: Operate the desktop shell across workspaces and connection states
persona: Bruno
journey:
expected: A fresh workspace renders one persistent desktop with menubar, dock, wallpaper, and command hint; workspace switching isolates complete window topologies; stream loss exposes an honest disconnected state, blocks unsafe mutations, and reconnect replaces the query cache from a new snapshot fence without regressing revision.
entry_points: web desktop root; workspace trigger; window-manager WebSocket stream
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-public-parity; ET-window-manager-multi-client; ET-web-window-routing-lifecycle; ET-web-menubar-menu-set
---

story: As a builder, I can see the authoritative desktop state, understand when it is disconnected, and recover without mixing workspace topology or revisions.

qa-impact: 2026-07-22 window-management hard cut replaced key-level hydration with snapshot-fenced Query reconciliation and client-scoped Zustand presentation; 2026-07-24 the win-layer reserves the Dock band (windows, fullscreen fills, and floating clamps stop above the Dock), reconciled active-desktop changes (zoom, cross-desktop focus, remote switches) synthesize the same keyframe slide as an explicit switch, and rapid commands queue behind the in-flight one instead of being silently dropped; the same-day review now remeasures work-area origin after orientation and Dock layout shifts, while queued commands reject an enqueue-time binding that no longer matches the active workspace/client; 2026-07-24 the menubar chrome became a real `role="menubar"` whose items include the AGH mark and the workspace chip (menu set tracked in `ET-web-menubar-menu-set`), so the workspace trigger is now a menu item rather than a plain button. Flag only; the next QA cycle owns live retesting.
