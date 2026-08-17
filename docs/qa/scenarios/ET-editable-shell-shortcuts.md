---
id: ET-editable-shell-shortcuts
area: ET
title: Keep palette and session creation available inside inputs
persona: Bruno
journey: J-operate-desktop-shell
expected: The live bindings for Command palette and New session still open their actions while an input is focused, question mark types normally, and Command slash opens the shortcut reference.
entry_points: web desktop keyboard; session composer; Settings inputs
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_05
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-command-palette-shortcuts
---

Flagged 2026-08-16 for the Herdr parity QA tail after the shell-chord registry migration.

QA 2026-08-16 Herdr parity: The full Web E2E, daemon settings contract suites, and inspected visual bundles covered editable shortcuts, array/range persistence, blocked and shadowed diagnostics, Terminal preset preview/apply/revert, live cheatsheet freshness, and editable-context routing.
