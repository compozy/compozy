---
id: MS-window-shortcut-arrays-ranges
area: MS
title: Rebind shortcuts with arrays and numbered ranges
persona: Bruno
journey: J-administer-window-manager
expected: Settings and config set persist ordered alternates, expand numbered ranges, name the exact conflicting member, and keep the prior effective keymap after a rejected save; a stored array or range bound to a retired action ID loads, does nothing, never counts as a conflict, and shows as a retired row in Settings.
entry_points: Settings › Layouts › Shortcuts; compozy config get/set window_manager.shortcuts; GET/PATCH /api/settings/window-manager over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: MS-configure-window-manager; ET-layout-editor-shortcut-recorder
---

Flagged 2026-08-16 for the Herdr parity QA tail. Cover array alternates, scalar and array ranges,
explicit disable, blocked duplicate, and surface-local shadow diagnostics in one settings journey.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

qa-impact: 2026-11-13 window-tabs-split S10. Retired action IDs (window.minimize,
window.zoom, window.toggle_floating, window.tile.top-left|top-right|bottom-left|bottom-right,
window.merge_all, window.tab.detach) are skipped by the effective keymap and by conflict checks.
Walk: preconditions — no overrides. 1) In the user config.toml under `[window_manager.shortcuts]` set
`"window.zoom" = ["control+alt+KeyZ", "alt+KeyY"]` and `"window.tab.jump" = "control+alt+Digit1..8"`,
then run `compozy config reload`: both load. 2) Settings ›
Layouts › Shortcuts: the jump family expands to eight rows with their chords; "Zoom window" shows
both chords with the "retired" flag. 3) Bind `alt+KeyY` to "Move to new tab": accepted, no conflict
named. 4) Bind `control+alt+Digit3` to another action: blocked, naming `window.tab.jump.3`. 5) Repeat
the original array, disable and shadow checks. Reset for the current build.
