---
id: MS-terminal-shortcut-preset
area: MS
title: Apply and revert the Terminal shortcut preset
persona: Bruno
journey: J-administer-window-manager
expected: The Terminal preset previews every displaced binding and platform hazard, applies as one valid change with Apply preset, re-applies without drift, restores the exact pre-preset overrides with Revert, and binds no retired action (no window.zoom or window.tile.* corner entry); Copy as TOML copies the same map the Settings preset applies.
entry_points: Settings › Layouts › Shortcuts › Terminal preset; shortcuts documentation
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: MS-window-shortcut-arrays-ranges; MS-configure-window-manager
---

Flagged 2026-08-16 for the Herdr parity QA tail. Include the documented TOML block as the
agent-manageable path and verify that its effective map matches the Settings preset.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.

qa-impact: 2026-11-13 window-tabs-split S10. The preset dropped `window.zoom` and
`window.tile.bottom-left|bottom-right`, which were retired with tabs and splits; its TOML block and
the docs page match. Walk: preconditions — no shortcut overrides. 1) Settings › Layouts › Shortcuts
› Terminal preset › Preview: the change list names only live actions (focus, close, tab
new/next/previous/jump/last, session and workspace entries) and none of window.zoom or
window.tile.*; hazards read "Control+Alt can alias AltGr on some layouts." and "Control+digits can be
reserved for browser tabs." where they apply. 2) Apply preset: the "Applied" pill appears, no row in
the shortcut table shows the "retired" flag, and `compozy config get window_manager.shortcuts -o json`
matches the preview. 3) Preview again: nothing to change. 4) Revert: the overrides return to empty.
5) Copy as TOML and compare with the Terminal preset block in the shortcuts docs. Reset for the
current build.
