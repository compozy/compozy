---
id: TA-web-task-list-loop-subtask-nesting
area: TA
title: Task list nests loop cell tasks under their coordinator row
persona: Dora
journey: J-05
expected: With an implement-tasks loop run in the workspace, the Tasks List view shows the coordinator task ("Loop coordinator <name>") as one row in its status group, and every loop cell task (parent `loop.<run>.…`) nests behind it instead of flooding the status sections. The row carries a collapsed summary line ("9 subtasks · 1 needs attention · 2 running …") that always counts escalations; clicking it expands compact child rows (status dot, short id like `g2.execute_task`, status + attempt, relative time; failed rows show the run error). Child rows link to the task detail. Cell tasks whose coordinator is not loaded on the page stay visible as top-level rows. The short id chip never renders the truncated `loop.lo` form; loop cells read `g<N>.<node>` and the coordinator reads `coordinator`. Loop cell rows show `attempt N` without the task-level `of 10` ceiling. Kanban shows only top-level cards. A node quarantined by the loop moves its cell task into Needs attention and a requeue returns it to Queued. The coordinator row stays Active for the whole life of the loop run — completed coordinator pulses between boundaries never flip it to Done, and it only reaches Done when the loop run itself ends and closes the task.
entry_points: web Tasks window -> List; web Tasks window -> Kanban
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-web-tasks-calm-default-reveal; TA-web-task-detail-loop-provenance; LP-web-run-attention-quarantine-routing; LP-web-run-session-one-click
---

retired — superseded by the server-owned exclusion contract (ADR-001, task_04). Loop execution records now leave every Tasks listing by default, so there is no coordinator row to nest cells behind and no client-side `buildTaskListTree`/`task-subtask-list` machinery left to walk; both were deleted with the id-regex identity they depended on. The behavior this scenario guarded is replaced by `TA-web-tasks-calm-default-reveal` (calm default, reveal filter, revealed-row grammar) and `TA-web-task-detail-loop-provenance` (structured provenance on the record's own page). The `blocked-decision` note below is frozen history — the dogfood run it waited on is no longer the way to walk this behavior.


story: As someone supervising a delivery loop I open Tasks to see my work, and the loop's mechanical cells must read as one collapsible unit under their coordinator — not eighteen near-identical rows drowning the list.

The nesting is client-side (`buildTaskListTree`): a child nests only when its parent is present on the page, so pagination never hides work. The collapsed summary leads with escalations (needs attention / blocked / failed) so collapsing never hides a child that needs someone. The backend now parks quarantined cell tasks in needs-attention (`markQuarantinedNodeTasks`) and requeue clears the park, so the status groups stay truthful.

blocked-decision: walking this requires a fresh implement-tasks dogfood run (spawns real ACP agent sessions on the operator's account); pending the operator starting one.

src: web/src/systems/tasks/components/task-card.tsx; web/src/systems/tasks/lib/task-formatters.ts; internal/loop/generation_snapshot_controls.go

inventory: Needs QA
