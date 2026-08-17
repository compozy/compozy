---
id: APP-desktop-page-zoom
area: APP
title: Scale and reset the desktop product with standard shortcuts
persona: Dora
journey: J-desktop-attach-daily
expected: In the daemon-served main desktop window, Command or Control plus, minus, and zero scale or reset the whole product page while the in-product single-window Zoom action remains separate and boot-window privileges do not expand.
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
reduces it, and Command-zero restores the original scale without invoking the in-product window Zoom
action.
