---
id: ET-web-empty-desktop-card
area: ET
title: An empty desktop says so and offers a way to start
persona: Bruno
journey: J-operate-desktop-shell
expected: A desktop with no visible windows shows one centered card on the flat desk: an app-window identity well, "{desktop name} is empty", "Open an app from the dock, or press ⌘K to open anything." with the live palette chord, and one primary New session button that starts a session on that desktop; the card never blocks the desk or the dock, disappears as soon as a window opens, returns when the last window closes or is minimized, and reads in both themes.
entry_points: web desk on a new or emptied desktop; New session button; ⌘K
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-desktop-shell-lifecycle; RT-desktop-pager-overview; ET-web-dock-contextual-session-launch
---

Added 2026-09-30 for the shell rail (VC-10 new empty desktop). Walk: create a desktop from the overview (card appears with its name), rebind the palette chord and confirm the copy follows it, start a session from the card, then close and minimize windows to bring the card back.
