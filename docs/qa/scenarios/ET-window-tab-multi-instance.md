---
id: ET-window-tab-multi-instance
area: ET
title: Cycle independent app instances without identity collisions
persona: Bruno
journey: J-organize-tabbed-work
expected: Opening multiple Tasks and Session instances assigns opaque identities, dock and context-menu destinations enumerate every live instance, repeated dock activation cycles in MRU order across desktops, minimized instances restore, and closing one instance never redirects or mutates another.
entry_points: web dock; dock app menu; command palette Go to tab; task and session deep links
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/keyboard-02-command-t-deck.png; docs/qa/evidence/2026-08-01-window-tabs/keyboard-03-palette-tabs.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-web-window-routing-lifecycle; ET-web-dock-default-window-size; ET-window-manager-multi-client
---

Derived from J-organize-tabbed-work step 1. Covers semantic identity, minimized and cross-desktop
edges, MRU continuity, and the adjacent dock/default-size regression surface.
