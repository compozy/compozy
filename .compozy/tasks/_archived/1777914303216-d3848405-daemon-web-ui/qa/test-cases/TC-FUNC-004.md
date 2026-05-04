## TC-FUNC-004: Task Board, Task Detail, and Related Run Context

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `bun run frontend:e2e`
- Supporting integration: `bunx vitest run --config web/vitest.config.ts web/src/routes/-workflow-tasks.integration.test.tsx`
**Automation Notes:** Playwright smoke validates the happy-path board/detail drill-down in the daemon-served browser. The route integration suite covers empty boards, invalid task IDs, and related-run rendering details.

### Objective

Verify that operators can inspect workflow task status through the task board, open a task detail page, and understand related run context without any in-browser authoring features.

### Preconditions

- [ ] Workflow inventory drill-down is working.
- [ ] Seeded workflow data contains at least one task board entry.
- [ ] The route integration fixtures remain aligned with the board/detail contract.

### Test Steps

1. From the workflow inventory, open the task board for `daemon-web-ui`.
   **Expected:** `task-board-view` renders with lanes/cards from the daemon payload.

2. Open a task from the board.
   **Expected:** `task-detail-view` renders with task metadata and status.

3. Verify related run context on task detail.
   **Expected:** Related runs are rendered when present, and the run link is navigable.

4. Run the supporting integration suite.
   **Expected:** Empty-board and invalid-task branches are covered and return clear UI states instead of silent failures.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Empty board | Workflow with zero tasks | `task-board-empty` renders |
| Invalid task ID | Missing/stale `task_id` | `task-detail-load-error` renders with transport message |
| Related run present | Task has related run summary | Run link and status render correctly |
| Related run absent | Task has no related runs | Empty related-run state is explicit |

### Related Test Cases

- `TC-FUNC-003`
- `TC-FUNC-007`

### Traceability

- TechSpec: "Backend Read Models", "Route Model"
- Task reference: `task_11`

### Notes

- This case is intentionally read-oriented. Any authoring/editing affordance is out of scope for v1 and should be treated as a defect in scope control.
