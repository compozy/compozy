---
id: ET-web-shortcut-keycap-legibility
area: ET
title: Shortcut key caps render one typeface at the kbd tier everywhere
persona: Sol
journey: J-operate-desktop-shell
expected: Every shortcut rendering — empty-desktop hint, palette footer hints, Settings shortcut rows and modifier pickers, the cheatsheet, composer hint, permission dock numeral, and site search hint — draws key glyphs (⌘ ⇧ ⌥ ⌃ ⏎ ⌫) and letters from the same typeface at the 10.5px/510 kbd tier; boxed caps share one bordered grammar and menu shortcut hints stay bare; the topbar palette control is a command icon whose tooltip carries the chords; nothing renders with mixed-font glyph fallback.
entry_points: web desktop topbar; empty desktop; command palette footer; Settings → Layouts shortcuts + behavior; shortcut cheatsheet; session composer
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-editable-shell-shortcuts; ET-live-shortcut-cheatsheet
---

Flagged 2026-08-20 for the shortcut key-cap root-cause pass: bundled Geist/JetBrains Mono subsets carry no key glyphs (⌘ ⇧ ⌥ ⌃ ⏎ ⌫ ⇥ ⎋ all fell back to system fonts with mismatched metrics inside 9–11px mono caps), so the pass introduced the `--font-keys` system-first stack plus the `--text-kbd`/`--tracking-kbd` tier, baked the bordered cap into the `Kbd` primitive, and routed every hand-rolled `<kbd>`/chip (topbar chip, modifier pickers, permission dock, composer, site search, menu shortcut slots) through the shared grammar.
