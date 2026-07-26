---
id: MS-configure-window-manager
area: MS
title: Configure window behavior and declarative layouts safely
persona: Bruno
journey: J-administer-window-manager
expected: Settings › Layouts exposes every supported `[window_manager]` value through direct manipulation — a desktop canvas, a 1:1 gap box, a snap-zone map, a repeat-width track, and a chord recorder — with no number field bound to a geometry value; out-of-range gaps, snap thresholds, history limit, duplicate repeat widths, and duplicate shortcut chords each name the exact value at fault and block the save while preserving the active known-good configuration; one floating save bar covers the global config and the workspace layout reviews inside its own card; a valid save hot-applies to the next command without restarting; workspace layout overrides remain isolated.
entry_points: Settings › Layouts; global config.toml; agh config get|set|apply; agh layout-profile list|get|put|delete
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-layout-recovery; ET-window-manager-layout-gestures
---

story: As an operator, I can tune window behavior and layouts without accepting a partial or internally conflicting runtime configuration.

qa-impact: 2026-07-22 replaced storage-limit settings with validated behavior defaults, shortcuts, bindings, gaps, snap thresholds, and declarative layout editing; 2026-07-24 added `window_manager.swap_modifier` (default `shift`) across config.toml, settings PATCH, Settings UI, and web gesture resolution; 2026-07-24 rebuilt Settings › Layouts as a direct-manipulation surface (canvas + inspector + docked review gate, diagram choice cards, gap box, snap map, repeat-width track, chord recorder, saved-layout cards) and added the `agh layout-profile` CLI verbs. Flag only; the next QA cycle owns live retesting.

QA impact 2026-07-25 (deep-review remediation): ratio-track controls now keep stable semantic
identity and layout JSON export removes its temporary anchor after download. Flag only; the next QA
cycle owns direct-manipulation and import/export retesting.
