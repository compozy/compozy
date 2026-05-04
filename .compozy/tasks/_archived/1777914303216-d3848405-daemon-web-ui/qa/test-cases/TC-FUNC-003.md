## TC-FUNC-003: Workflow Sync, Archive, and Drill-Down

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
- Supporting request-shape checks: `bunx vitest run --config web/vitest.config.ts web/src/routes/-app-shell.integration.test.tsx`
**Automation Notes:** Playwright smoke validates the real sync and archive controls against the daemon-served UI, while the route integration suite confirms the correct workspace-scoped request payloads are sent.

### Objective

Verify that the workflow inventory exposes working sync and archive controls, and that operators can drill from the inventory into workflow-specific surfaces without leaving the daemon-served UI.

### Preconditions

- [ ] Playwright smoke fixture contains `daemon-web-ui` and `archive-ready` workflows.
- [ ] The active workspace is resolved.
- [ ] Archive-ready workflow is present and syncable before archive is attempted.

### Test Steps

1. Run the Playwright smoke suite and open `/workflows`.
   **Expected:** `workflow-inventory-view` is visible.

2. Trigger sync for `daemon-web-ui`.
   **Expected:** A success message confirms the workflow was synced.

3. Use the workflow board drill-down link from the inventory.
   **Expected:** The task board route loads for the selected workflow.

4. Trigger sync for `archive-ready`, then archive it.
   **Expected:** Success messaging confirms both actions, and the archived workflow appears under the archived inventory section.

5. Run the supporting route integration suite if root-cause evidence is needed.
   **Expected:** The sync request includes the active workspace and targeted workflow slug, and archive uses the daemon contract rather than local-only state.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Sync success | Valid active workflow | Success banner/toast appears |
| Archive success | Seeded completed workflow | Workflow moves into archived section |
| Inventory drill-down | Open board from workflow card | Task board route loads without layout breakage |
| Request wiring | Active workspace header/body | Workspace context is preserved in the mutation |

### Related Test Cases

- `TC-FUNC-002`
- `TC-FUNC-004`

### Traceability

- TechSpec API endpoints: `/api/sync`, `/api/tasks/:slug/archive`
- Task references: `task_09`, `task_11`, `task_14`, `task_15`

### Notes

- Execution should treat archive as a real daemon mutation, not a mocked UI-only transition.
