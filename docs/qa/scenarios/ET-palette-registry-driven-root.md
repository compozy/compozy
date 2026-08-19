---
id: ET-palette-registry-driven-root
area: ET
title: Command palette renders every row from the daemon registry
persona: Bruno
journey: J-operate-command-palette
expected: Command-K opens instantly against the last-known catalog and lists apps, shell, window, tab, layout and settings commands sourced from the daemon registry with their effective chords; a command unavailable in context stays visible and disabled carrying the runtime's own reason verbatim, a command irrelevant to this surface is absent rather than dead, a capped group states the exact overflow, and the same id, label and chord appear on the palette row, the menubar item, the cheatsheet line and the settings shortcut table.
entry_points: Command-K; menubar Go/Window/Session/Help menus; Help > Keyboard shortcuts; Settings > Layouts > Shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-tab-palette-search; ET-web-command-palette-shortcuts; ET-palette-nested-views
---

Covers the P1 web absorption: one registry projection behind every command surface, availability
resolved against this client's context, and honest degradation when the daemon is cold or
reconnecting. Walk the disabled-with-reason and cross-surface parity paths explicitly — they are the
invariants the projection exists to hold.
