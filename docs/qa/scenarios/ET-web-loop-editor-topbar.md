---
id: ET-web-loop-editor-topbar
area: ET
title: Loop editor chrome uses shell topbar actions
persona: Bruno
journey:
expected: `/loops/:name/editor` shows a single shell Topbar with breadcrumb on the left (Home › Loops › name › Editor) and Validate / Save layout / Publish plus version/dirty chips on the right via `useTopbarSlot`. There is no secondary identity bar with pencil + "title · Edit". The canvas sub-toolbar (zoom, Graph|DSL, invariant chips) remains below the topbar.
entry_points: web /loops/:name/editor
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-086; ET-web-route-chrome-topbar
---

Added by loop editor topbar chrome collapse. Flag only — retest in the next QA cycle.
