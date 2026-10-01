---
id: ET-web-dock-foot-and-compact-tab-bar
area: ET
title: Reach profile, theme, and Settings from the dock foot, also on compact screens
persona: Bruno
journey: J-operate-desktop-shell
expected: The dock foot (below the launchers, no divider) holds the profile switcher ("Profile: {name}"), the theme toggle ("Switch to light mode" / "Switch to dark mode", tooltip "Light mode" / "Dark mode"), and Settings, in that order, each with a right-side tooltip; the topbar tray no longer carries Settings or the profile switcher; below 960px the rail hides and a bottom tab bar keeps every launcher (horizontal scroll) with the same three foot controls behind a hairline at its end and no New session button; every control works by keyboard and names itself for screen readers in both themes.
entry_points: web dock foot; web compact (<960px) bottom tab bar; keyboard Tab/Up/Down in the dock
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-theme-toggle-persists; ET-profile-switcher-restore; ET-web-menubar-menu-set
---

Added 2026-09-30 for the shell rail (D2 compact, D6 rail foot). Walk at 1440×900 and at 800×900: open the profile switcher and Settings from the foot, flip the theme, and confirm the compact tab bar carries the same foot controls and every launcher.
