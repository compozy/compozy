---
id: TA-web-task-list-loop-subtask-nesting
area: TA
title: Task list nests loop cell tasks under their coordinator row
persona: Dora
journey: J-05
expected: With a software-delivery loop run in the workspace, the Tasks List view shows the coordinator task ("Loop coordinator <name>") as one row in its status group, and every loop cell task (parent `loop.<run>.…`) nests behind it instead of flooding the status sections. The row carries a collapsed summary line ("9 subtasks · 1 needs attention · 2 running …") that always counts escalations; clicking it expands compact child rows (status dot, short id like `g2.execute_task`, status + attempt, relative time; failed rows show the run error). Child rows link to the task detail. Cell tasks whose coordinator is not loaded on the page stay visible as top-level rows. The short id chip never renders the truncated `loop.lo` form; loop cells read `g<N>.<node>` and the coordinator reads `coordinator`. Loop cell rows show `attempt N` without the task-level `of 10` ceiling. Kanban shows only top-level cards. A node quarantined by the loop moves its cell task into Needs attention and a requeue returns it to Queued.
entry_points: web Tasks window -> List; web Tasks window -> Kanban
qa_status: blocked-decision
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-web-run-attention-quarantine-routing; LP-web-run-session-one-click
---

story: As someone supervising a delivery loop I open Tasks to see my work, and the loop's mechanical cells must read as one collapsible unit under their coordinator — not eighteen near-identical rows drowning the list.

The nesting is client-side (`buildTaskListTree`): a child nests only when its parent is present on the page, so pagination never hides work. The collapsed summary leads with escalations (needs attention / blocked / failed) so collapsing never hides a child that needs someone. The backend now parks quarantined cell tasks in needs-attention (`markQuarantinedNodeTasks`) and requeue clears the park, so the status groups stay truthful.

blocked-decision: walking this requires a fresh software-delivery dogfood run (spawns real ACP agent sessions on the operator's account); pending the operator starting one.

src: web/src/systems/tasks/lib/task-hierarchy.ts; web/src/systems/tasks/components/task-subtask-list.tsx; web/src/systems/tasks/components/task-card.tsx; web/src/systems/tasks/lib/task-formatters.ts; internal/loop/generation_snapshot_controls.go

inventory: Needs QA
