---
id: TA-web-tasks-calm-default-reveal
area: TA
title: Web Tasks stays work-items only and reveals Loop records on request
persona: Dora
journey: J-supervise-loop-steady-state
expected: With a Loop run in the workspace, the web Tasks list and board show zero Loop execution records and group counts that match their rows; the toolbar reveal reads "Work items" until turned on, and turning it on adds coordinator and cell rows carrying a loop glyph, a plain identity ("<loop> · run" / "<loop> · round N · step <node>"), a "Loop run"/"Loop step" tag and a link that lands on that run's page; leaving Tasks and coming back turns the reveal back off; revealing in a workspace with no Loop records shows "No loop records in this workspace" with a "Show work items" action instead of the generic empty; a record whose run retention deleted reads "Run no longer available" with its run id and no link; a workspace holding only Loop records shows the true "No tasks yet" empty.
entry_points: web Tasks window -> List; web Tasks window -> Kanban; web Tasks window -> Dashboard
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-task-list-calm-loop-default; TA-web-task-detail-loop-provenance
---

story: As someone supervising a delivery loop I open Tasks to see my work, and the loop's mechanical records must not be there — but they must still be findable when I go looking for them.

Exclusion is server-owned (`include_loop` absent means excluded), so counts, facets and every page of results agree by construction; there is no client-side filtering left to trim a loaded page. Walk the default read first and confirm the board is the same population as the list, then turn the reveal on and check the row grammar: structure is carried by the glyph and the role tag, status by the trailing pill, and the machine id never leads. The reveal is deliberately ephemeral — it is not a `config.toml` key and not a URL parameter — so confirm it resets on navigation rather than persisting.

Both empty states matter and are different sentences: the reveal-scoped one names the filter and offers the way out; the true empty says there is no work yet.

src: web/src/systems/tasks/components/tasks-list-surface.tsx; web/src/systems/tasks/components/task-loop-row.tsx; web/src/systems/tasks/components/tasks-list-records-filter.tsx; web/src/systems/tasks/hooks/use-tasks-page.ts; web/src/systems/tasks/lib/task-loop-identity.ts

inventory: Needs QA
