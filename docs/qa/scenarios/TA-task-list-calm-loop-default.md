---
id: TA-task-list-calm-loop-default
area: TA
title: Keep task listings calm while preserving typed Loop drilldown
persona: Ada
journey: J-operate-loop-run-headless
expected: During an active Loop, default task catalogs, counts, dashboard totals, and inbox lanes omit coordinator and cell records on HTTP, UDS, CLI, and native tools; typed include and run filters reveal the same records with structured provenance, while a parent drilldown still returns its children.
entry_points: web Tasks window under All projects (List, Kanban, Inbox, Dashboard, Project filter); compozy task list; compozy task list --include-loop; compozy task list --loop-run <run-id>; compozy task list --parent <task-id>; GET /api/tasks over HTTP and UDS; GET /api/tasks/:id over HTTP and UDS; compozy__task_list; task dashboard and inbox; skills/compozy/references/tasks-and-orchestration.md; /docs/cli/task/list
qa_status: untested
bug_ids:
fix_status:
retest_status: untested
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/cross-surface/default-parity.sha256; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/cross-surface/include-parity.sha256; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/cross-surface/loop-run-parity.sha256
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: TA-web-task-list-loop-subtask-nesting
---

Walk one workspace containing ordinary work plus an active Loop across every public listing surface. Compare the default and typed include results, scope to one run, open a coordinator's children, and confirm catalog facets, dashboard totals, and inbox counts follow the visible population. Repeat a structured single-task read for a cell and coordinator and compare their `loop` objects with the catalog rows.

Documentation is part of this contract, not a separate check: the official skill's task-listing reference and the generated `task list` CLI page must describe the calm default and the two typed flags exactly as the runtime behaves. An agent that reads a wrong flag from the docs is as broken as a wrong response.

QA impact 2026-08-21: task_06 added the typed flags, the single-task read, and the docs/skill surface as entry points on this row rather than minting a separate docs scenario — coverage belongs on the journey-derived row.

QA result 2026-08-21: CLI, HTTP, UDS, and session-scoped `compozy__task_list` produced identical
normalized catalogs. The calm default returned 12 ordinary tasks; typed reveal returned 17 rows,
including five Loop records. `--loop-run` implied inclusion, the unknown filter returned empty at
exit 0, and invalid `include_loop` returned field-addressed 400 over HTTP and UDS.

QA impact 2026-10-21 (All projects backend): repeat the ordinary-work catalog and dashboard
checks with `all_workspaces=true`; omission retains its existing population. Narrow with repeated
`workspace_ids` values, including `@user`, and verify totals and facets before paging. A cursor from
another project selection or profile must fail. `all_workspaces=true` conflicts with `workspace`
and with a bounded `scope`. Compare `workspace_ids=@user` with the existing `scope=global` population.
The CLI `task list --all-workspaces` shows a PROJECT column and labels projectless tasks User.
Overview omission still selects global tasks; `all_workspaces=true` selects every task. App tokens
retain task catalog/dashboard access and remain denied on overview. Ordinary agent identities cannot
request the explicit aggregate. The unrestricted-agent exception belongs to the prerequisite grant slice.

Backend evidence: [2026-10-21 All projects task slice](../reports/2026-10-21-all-projects-user-scope.md).
The Web All projects journey remains an integration check for the owning frontend slice.

2026-10-21 qa-impact (all-projects AW2, web Tasks): **reset to `untested`** for the web consumer.
Under All projects the Tasks list, Kanban, and Dashboard read `all_workspaces=true` (never
`scope=global`); the scheduler backlog reads `scope=all`; the Inbox omits the scope (its omission is
every task). Each row, Kanban card, Inbox item, and active run shows its project badge ("User" for a
projectless task) and opens the task in place on the All projects desktop with the project badge in
the window chrome. The list toolbar gains a **Project** filter (multi-select, User last) that narrows
server-side with repeated `workspace_ids`, resets when All projects turns off or back on, and is hidden
in project mode; a filter that empties the list shows "No tasks match the current filters" with
Clear filters. The task dock badge counts awaiting-approval tasks across every project under All
projects and only the focused project (or User) otherwise. Also open a task from the bell and from the
⌘K Tasks section: both stay in All projects. The calm default and the loop reveal keep their
behaviour inside the aggregate.
