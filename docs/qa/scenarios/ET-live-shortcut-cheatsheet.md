---
id: ET-live-shortcut-cheatsheet
area: ET
title: Read the effective keymap immediately after rebinding
persona: Bruno
journey: J-operate-desktop-shell
expected: The keyboard reference opens from either live binding and immediately shows daemon defaults, overrides, alternates, compact numbered ranges, disables, and locked surface-local controls without duplicate rows.
entry_points: Help › Keyboard shortcuts; question mark; Command slash
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-shell-shortcuts-about-dialogs; ET-layout-editor-shortcut-recorder
---

Flagged 2026-08-16 for the Herdr parity QA tail. The true end state is a fresh cheatsheet read after
a saved rebind without reloading the shell.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.
