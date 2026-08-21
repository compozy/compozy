---
id: TA-task-list-calm-loop-default
area: TA
title: Keep task listings calm while preserving typed Loop drilldown
persona: Ada
journey: J-operate-loop-run-headless
expected: During an active Loop, default task catalogs, counts, dashboard totals, and inbox lanes omit coordinator and cell records on HTTP, UDS, CLI, and native tools; typed include and run filters reveal the same records with structured provenance, while a parent drilldown still returns its children.
entry_points: compozy task list; compozy task list --include-loop; compozy task list --loop-run <run-id>; compozy task list --parent <task-id>; GET /api/tasks over HTTP and UDS; GET /api/tasks/:id over HTTP and UDS; compozy__task_list; task dashboard and inbox; skills/compozy/references/tasks-and-orchestration.md; /docs/cli/task/list
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

Documentation is part of this contract, not a separate check: the official skill's task-listing reference and the generated `task list` CLI page must describe the calm default and the two typed flags exactly as the runtime behaves. An agent that reads a wrong flag from the docs is as broken as a wrong response.

QA impact 2026-08-21: task_06 added the typed flags, the single-task read, and the docs/skill surface as entry points on this row rather than minting a separate docs scenario — coverage belongs on the journey-derived row.
