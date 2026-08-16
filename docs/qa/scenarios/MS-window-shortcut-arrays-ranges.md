---
id: MS-window-shortcut-arrays-ranges
area: MS
title: Rebind shortcuts with arrays and numbered ranges
persona: Bruno
journey: J-administer-window-manager
expected: Settings and config set persist ordered alternates, expand numbered ranges, name the exact conflicting member, and keep the prior effective keymap after a rejected save.
entry_points: Settings › Layouts › Shortcuts; compozy config get/set window_manager.shortcuts; GET/PATCH /api/settings/window-manager over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager; ET-layout-editor-shortcut-recorder
---

Flagged 2026-08-16 for the Herdr parity QA tail. Cover array alternates, scalar and array ranges,
explicit disable, blocked duplicate, and surface-local shadow diagnostics in one settings journey.
