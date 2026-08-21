---
id: ET-desktop-global-summon
area: ET
title: Summon a command from anywhere with a desktop global hotkey
persona: Bruno
journey: J-command-os-from-palette
expected: A confirmed desktop-global chord restores and focuses CompozyOS, opens the palette or the command's argument step, and does not execute through an existing modal. A captured replacement keeps the previous confirmed chord active and names the conflict; relaunch registers and reports fresh truth. Plain browsers explain that the feature requires the desktop shell while the in-app palette chord remains available.
entry_points: desktop-global hotkey (default meta+shift+Space); Settings > Layouts > Shortcuts global section; compozy cmd-palette bind <id> <chord> --global / unbind <id> --global; [window_manager.global_shortcuts] in config.toml
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

Walk (task_11 plan):

1. With the desktop shell running and another app focused, press the summon chord — the Compozy
   window restores/focuses with the palette open; over an open modal, the window focuses without
   executing through it.
2. Bind a global chord to an argument-bearing command (`cmd-palette bind <id> <chord> --global`) —
   firing it unfocused summons the window with the palette already in that command's argument step.
3. Pre-claim a chord in another app, then try to take it — Settings shows "unavailable — in use by
   another application", the previous confirmed chord stays active, and a chord renders active only
   once its registration is confirmed.
4. Quit and relaunch the shell — registrations release, re-register, and re-report fresh truth;
   `unbind --global` removes the intended binding from `[window_manager.global_shortcuts]`.
5. Open the same web app in a plain browser — the global section explains "requires desktop shell"
   while the in-app ⌘K chord keeps working; on macOS, the accessibility requirement surfaces with a
   System Settings deep-link instead of failing silently.

Expected evidence: screen captures of the summon from another app, the argument-step summon, the
in-use failure state with the previous chord still live, and the browser-mode reason; the
bind/unbind --global transcripts and the config section before/after.
