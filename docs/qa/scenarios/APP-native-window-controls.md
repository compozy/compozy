---
id: APP-native-window-controls
area: APP
title: Operate the desktop window from native controls in the menubar
persona: Dora
journey: J-desktop-attach-daily
expected: The packaged product window shows operating-system window controls within the 44px Compozy menubar without covering product controls; macOS keeps native traffic lights before the Compozy mark, Linux follows the desktop environment's native side and button set, and the browser renders the unchanged menubar without desktop controls.
entry_points: packaged macOS product window; packaged Linux product window; web desktop in a browser
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-native-window-chrome-20260817-190228-135313-lab/qa-artifacts/qa; docs/qa/reports/2026-08-17-native-window-chrome.md
last_report: docs/qa/reports/2026-08-17-native-window-chrome.md
overlaps: APP-window-geometry-recovery; APP-quit-contract; ET-web-menubar-menu-set
---

Added 2026-08-17 for native Electron window chrome. The controls remain owned by the operating
system; the renderer only reserves the Window Controls Overlay safe area and draggable menubar.

2026-08-17 QA: packaged macOS and browser fallback passed. Linux remains blocked until the same
charter is walked on a Linux desktop environment, where the control side and button set are owned
by the window manager.
