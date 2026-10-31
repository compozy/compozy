---
id: APP-native-window-controls
area: APP
title: Operate the desktop window from native controls in the topbar
persona: Dora
journey: J-desktop-attach-daily
expected: The packaged product window shows operating-system window controls within the 52px CompozyOS topbar without covering product controls; macOS traffic lights sit inside the 84px leading reserve before the CompozyOS mark, vertically centred in the bar; Linux follows the desktop environment's native side and button set, painted on the topbar color with symbols that follow the light or dark theme; the whole topbar drags the window while every control in it stays clickable; in-product windows keep their own quiet trailing minimize, zoom and close icon buttons, distinct from the native controls; the browser renders the topbar without the reserve or desktop controls.
entry_points: packaged macOS product window; packaged Linux product window; web desktop in a browser
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-native-window-chrome-20260817-190228-135313-lab/qa-artifacts/qa; docs/qa/reports/2026-08-17-native-window-chrome.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: APP-window-geometry-recovery; APP-quit-contract; ET-web-menubar-menu-set; APP-theme-native-no-flash
---

Added 2026-08-17 for native Electron window chrome. The controls remain owned by the operating
system; the renderer only reserves the Window Controls Overlay safe area and draggable menubar.

2026-08-17 QA: packaged macOS and browser fallback passed. Linux remains blocked until the same
charter is walked on a Linux desktop environment, where the control side and button set are owned
by the window manager.

2026-09-30 (shell rail v2): the menubar became the 52px topbar and the dock a left rail. macOS sets
`trafficLightPosition` so the lights centre in the 84px reserve (`--width-traffic-lights`); Linux's
`titleBarOverlay` uses the rail color and a theme-following symbol color, recolored live on a theme flip.
The earlier macOS/browser pass covered the 44px menubar, so the scenario is back to untested; Linux is
still unwalked.

Development evidence (not a QA run, 2026-09-30, macOS dev Electron): traffic lights measured at x 15–74.5pt
(inside the 84pt reserve) and y 19–32.5pt (centre 25.75pt in the 52pt bar); the topbar `<header>` is
`app-region: drag` and every control in it is `no-drag` itself or under a `no-drag` group (menus, tray,
globe). Real OS clicks and drags were not exercised (the machine was in live use).


2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

qa-impact: 2026-10-31 (#1094) the Skip to content link popped up during mouse use under the macOS
traffic lights: showing the window again (⌘H and back, Dock reopen) made Chromium hand initial focus to
the page's first tabbable element as keyboard focus. By the user's decision the skip link is removed;
the menubar and rail tab order is the keyboard path. With the link gone the same hand-off lands on the
CompozyOS mark and paints its focus ring, so the shell now releases focus that arrives on window
activation before any key or pointer input. Expected: after hiding and re-showing the window with the
mouse, no control in the topbar shows a focus ring. Development evidence only; the packaged walk remains
outstanding.
