---
id: ET-web-loop-editor-sidebar-tabs
area: ET
title: Loop editor right rail uses Contract / Node lane tabs
persona: Bruno
journey:
expected: On `/loops/:name/editor`, the right rail shows LaneTabs with Contract (default) and Node. Contract fields (goal, definition of done) are visible on open. Selecting a canvas node, adding from the palette, or revealing from the linter switches to the Node tab and shows that node's inspector. Manual tab clicks switch lanes without clearing canvas selection.
entry_points: web /loops/:name/editor
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-loop-editor-topbar; LP-toggle-loop-goal
---

Added by loop editor Contract/Node lane tabs. Flag only — retest in the next QA cycle.
