---
id: ET-web-theme-system-follows-os
area: ET
title: Follow the operating system's light or dark setting live
persona: Bruno
journey: J-operate-desktop-shell
expected: With Appearance > Theme set to System, the desktop paints the OS scheme at load and repaints live when the OS switches, with no reload; open toasts, the loop editor canvas, native form controls and a running terminal repaint with it; pressing the rail-foot toggle while on System stores the explicit opposite of what is painted and stops following the OS.
entry_points: Settings > Appearance > Theme (System); OS appearance setting; rail-foot theme toggle
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-theme-toggle-persists; APP-theme-native-no-flash
---

Added 2026-09-30 for the shell-rail theme (D4). `system` resolves through `prefers-color-scheme`; in the
desktop app the stored preference becomes `nativeTheme.themeSource`, so `system` there also follows the OS.

Walk: choose System, flip the OS appearance both ways with a toast visible, the loop editor open, a cron
builder time input on screen and a live terminal showing ANSI colors; each surface repaints in place.
Then press the rail toggle and flip the OS again: nothing changes (the choice is now explicit).

Development evidence (not a QA run): on the shell-rail lab, toasts (`data-sonner-theme`), the loop editor
(`.react-flow` `dark` → `light`) and a live zsh terminal repainted on a flip; the cron builder time input's
`color-scheme` follows the theme in Storybook.
