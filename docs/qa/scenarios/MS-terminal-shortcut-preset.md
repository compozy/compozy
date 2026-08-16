---
id: MS-terminal-shortcut-preset
area: MS
title: Apply and revert the Terminal shortcut preset
persona: Bruno
journey: J-administer-window-manager
expected: The Terminal preset previews every displaced binding and platform hazard, applies as one valid change, re-applies without drift, and restores the exact pre-preset overrides in one step.
entry_points: Settings › Layouts › Shortcuts › Terminal preset; shortcuts documentation
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-window-shortcut-arrays-ranges; MS-configure-window-manager
---

Flagged 2026-08-16 for the Herdr parity QA tail. Include the documented TOML block as the
agent-manageable path and verify that its effective map matches the Settings preset.
