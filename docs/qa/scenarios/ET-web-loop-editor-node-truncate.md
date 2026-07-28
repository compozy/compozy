---
id: ET-web-loop-editor-node-truncate
area: ET
title: Loop editor node cards truncate long id and kind labels
persona: Bruno
journey:
expected: On `/loops/:name/editor`, ACTION (and other) canvas node cards keep id and kind text inside the fixed-width card with ellipsis; hover `title` exposes the full string. Labels do not paint outside the node border.
entry_points: web /loops/:name/editor
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-021; ET-web-loop-editor-topbar
---

Added by loop editor node label truncate. Flag only — retest in the next QA cycle.
