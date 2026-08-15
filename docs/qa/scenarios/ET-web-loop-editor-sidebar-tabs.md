---
id: ET-web-loop-editor-sidebar-tabs
area: ET
title: Loop editor right rail uses Contract / Node lane tabs
persona: Bruno
journey: J-06
expected: On `/loops/:name/editor`, the right rail shows LaneTabs with Contract (default) and Node. Contract fields (goal, definition of done) are visible on open. Selecting a canvas node, adding from the palette, or revealing from the linter switches to the Node tab and shows that node's inspector. Manual tab clicks switch lanes without clearing canvas selection.
entry_points: web /loops/:name/editor
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: b17c0dfb
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-web-loop-editor-topbar; LP-toggle-loop-goal
---

Added by loop editor Contract/Node lane tabs. Flag only — retest in the next QA cycle.

Reset 2026-08-14: both lanes now share one field-label grammar (Field/RequiredMark), folds carry rotating chevrons with reliability and wait-expiry open by default, and Reveal node also centers the viewport (b17c0dfb). Still awaiting the seeded QA walk; editor suites green at 9a694ff2.
