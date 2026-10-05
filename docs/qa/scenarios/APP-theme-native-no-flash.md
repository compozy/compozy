---
id: APP-theme-native-no-flash
area: APP
title: Open the desktop app in the chosen theme without a flash
persona: Dora
journey: J-desktop-attach-daily
expected: The desktop app opens its boot window and product window already painted in the stored theme (dark by default, light after the user chose light) with no frame of the other theme, including while resizing; the boot page follows the same theme; flipping the theme in the product repaints the native window background and, on Linux, the window controls; with System the app follows the OS live; the choice survives quitting and relaunching.
entry_points: packaged desktop app launch; rail-foot theme toggle; Settings > Appearance > Theme; OS appearance setting; app quit and relaunch
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-theme-toggle-persists; ET-web-theme-system-follows-os; APP-native-window-controls; APP-window-geometry-recovery
---

Added 2026-09-30 for the shell-rail theme in Electron. The renderer owns the preference and reports it
over the product bridge (`theme.set`); main stores it in `<COMPOZY_HOME>/desktop-theme.json` and sets
`nativeTheme.themeSource` before any window exists. Window backgrounds match each page's first surface:
product chrome `#0a0a0a` / `#fafafa`, boot canvas `#1a1a1a` / `#ffffff`.

Walk: fresh home → launch (dark boot, dark product); toggle to light; quit; relaunch and record the
boot and product windows (screen recording, no dark frame); live-resize the product window (edges stay
light); choose System and flip the OS appearance; on Linux confirm the title-bar controls recolor.

Development evidence (not a QA run, 2026-09-30, macOS, dev Electron attached to the shell-rail demo
daemon): burst screen captures at ~8 fps show no frame of the other theme for the dark default (boot
`#1a1a1a`, product first frame `#0a0a0a`) or after a stored light relaunch (boot `#ffffff`, product first
frame `#fafafa`); one toggle moved `nativeTheme.themeSource` to `light`, the native background to
`#FAFAFA` and wrote `desktop-theme.json`. Artifacts: `.compozy/tasks/shell-rail/reports/theme-B3/`.
Linux overlay recolor unwalked.


2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
