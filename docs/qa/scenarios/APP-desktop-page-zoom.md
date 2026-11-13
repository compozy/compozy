---
id: APP-desktop-page-zoom
area: APP
title: Scale and reset the desktop product with standard shortcuts
persona: Dora
journey: J-desktop-attach-daily
expected: In the daemon-served main desktop window, Command or Control plus, minus, and zero scale or reset the whole product page without changing the desktop's tabs or splits, and boot-window privileges do not expand.
entry_points: native desktop main window keyboard shortcuts
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: ET-window-manager-layout-gestures
---

Introduced for issue #345. The permission is limited to the remote daemon-served `main` window and standard loopback origins.

The Electron walk must prove that Command-plus enlarges the entire product page, Command-minus
reduces it, and Command-zero restores the original scale without changing the active tab or any split.

qa-impact: 2026-11-13 window-tabs-split S10. The in-product Zoom window action is retired, so the page-zoom shortcuts can no longer collide with it. Expected and the walk sentence now check that page zoom leaves tabs and splits unchanged. The page-zoom contract this scenario verifies is otherwise unchanged; status unchanged.
