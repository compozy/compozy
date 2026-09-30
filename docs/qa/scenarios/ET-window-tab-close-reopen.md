---
id: ET-window-tab-close-reopen
area: ET
title: Close tab scopes and restore the newest entry
persona: Théo
journey: J-organize-tabbed-work
expected: Tab, right, others, and group close scopes remove exactly the intended unpinned members in one command and one closed entry; multi-level reopen restores ids, routes, pins, order, placement, and destination fallback after a full reload while minimize records nothing.
entry_points: tab context menu; tab close button; window Close control; Command-Shift-T; compozy window close|reopen
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/supervisor-02-recovery-desktop.png
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-public-parity; ET-web-window-routing-lifecycle
---

Derived from J-organize-tabbed-work step 4 and its reload aftermath. Covers ADR-013, destructive
scope clarity, history bounds, missing-desktop fallback, and the TechSpec invariant 13 hot spot.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). Tab close moved to the 20px close button on browser-style tabs and window close to the quiet Close icon control. Reset to re-walk the close scopes and multi-level reopen from the new controls.
