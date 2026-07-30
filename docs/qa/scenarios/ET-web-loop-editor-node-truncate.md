---
id: ET-web-loop-editor-node-truncate
area: ET
title: Loop editor node cards truncate long id and kind labels
persona: Bruno
journey: J-06
expected: On `/loops/:name/editor`, ACTION (and other) canvas node cards keep id and kind text inside the fixed-width card with ellipsis; hover `title` exposes the full string. Labels do not paint outside the node border.
entry_points: web /loops/:name/editor
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: LP-021; ET-web-loop-editor-topbar
---

Added by loop editor node label truncate. Flag only — retest in the next QA cycle.
