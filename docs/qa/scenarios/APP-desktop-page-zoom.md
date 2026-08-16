---
id: APP-desktop-page-zoom
area: APP
title: Scale and reset the desktop product with standard shortcuts
persona: Dora
journey: J-desktop-attach-daily
expected: In the daemon-served main desktop window, Command or Control plus, minus, and zero scale or reset the whole product page while the in-product single-window Zoom action remains separate and boot-window privileges do not expand.
entry_points: native desktop main window keyboard shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/desktop-zoom-in-pass.png;/Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/desktop-zoom-out-pass.png;/Users/pedronauck/dev/qa-labs/compozy-open-issues-20260812-002435-338441-lab/qa-artifacts/desktop-zoom-reset-pass.png
last_report: docs/qa/reports/2026-08-11-open-issues.md
overlaps: ET-window-manager-layout-gestures
---

Introduced for issue #345. The permission is limited to the remote daemon-served `main` window and standard loopback origins.

2026-08-11 retest: passed in the native Tauri shell. Command-plus enlarged the entire product page, Command-minus reduced it, and Command-zero restored the original scale without invoking the in-product window Zoom action.
