---
id: ET-desktop-global-summon
area: ET
title: Summon a command from anywhere with a desktop global hotkey
persona: Bruno
journey: J-operate-command-palette
expected: A confirmed desktop-global chord restores and focuses CompozyOS, opens the palette or the command's argument step, and does not execute through an existing modal. A captured replacement keeps the previous confirmed chord active and names the conflict; relaunch registers and reports fresh truth. Plain browsers explain that the feature requires the desktop shell while the in-app palette chord remains available.
entry_points: desktop-global hotkey; Settings > Layouts > Shortcuts; command palette global binding
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-inline-args-confirmation; ET-web-command-palette-shortcuts; ET-live-shortcut-cheatsheet
---

Flagged by command-palette task 09. Task 12 owns the first real-user walk, E2E-027–030,
visual-contract comparison, and verdict.
