---
id: TA-task-list-calm-loop-default
area: TA
title: Keep task listings calm while preserving typed Loop drilldown
persona: Ada
journey: J-configure-and-run-loop
expected: During an active Loop, default task catalogs, counts, dashboard totals, and inbox lanes omit coordinator and cell records on HTTP, UDS, CLI, and native tools; typed include and run filters reveal the same records with structured provenance, while a parent drilldown still returns its children.
entry_points: compozy task list; GET /api/tasks over HTTP and UDS; compozy__task_list; task dashboard and inbox
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-task-list-loop-subtask-nesting
---

Walk one workspace containing ordinary work plus an active Loop across every public listing surface. Compare the default and typed include results, scope to one run, open a coordinator's children, and confirm catalog facets, dashboard totals, and inbox counts follow the visible population. Repeat a structured single-task read for a cell and coordinator and compare their `loop` objects with the catalog rows.
