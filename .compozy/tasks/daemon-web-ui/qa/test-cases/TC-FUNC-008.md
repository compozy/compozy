## TC-FUNC-008: Workflow Run Start from the Browser Surface

**Priority:** P1 (High)
**Type:** Functional
**Status:** Active
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- E2E smoke: `bun run frontend:e2e` (`web/e2e/daemon-ui.smoke.spec.ts`)
- Supporting integration seam: `bunx vitest run --config web/vitest.config.ts web/src/routes/-workflow-tasks.integration.test.tsx web/src/systems/workflows/components/workflow-inventory-view.test.tsx`
**Automation Notes:** `task_16` wired the workflow inventory to `useStartWorkflowRun`, validated the browser request against the daemon-served UI, and corrected the Playwright fixture so the smoke case no longer collides with a seeded `daemon-web-ui` run.

### Objective

Validate that a real browser operator can start a workflow run from the daemon-served workflow inventory and reach a visible success state without leaving `/workflows`.

### Preconditions

- [ ] The execution branch includes the final `task_10` UI surfaces.
- [ ] The route tree and systems directories are searchable.
- [ ] Browser/operator QA must still treat run start as a required flow.

### Test Steps

1. Inspect the dashboard, workflow inventory, task detail, reviews, and runs surfaces for a workflow start control.
   **Expected:** Either a visible operator control exists, or the absence is confirmed explicitly.

2. Search the route and system code for `useStartWorkflowRun` or other UI wiring to `/api/tasks/{slug}/runs`.
   **Expected:** If the flow is wired, the owning route/component is identified; if not, the gap is confirmed.

3. Run the supporting adapter test.
   **Expected:** The POST contract itself is still valid even if the browser entrypoint is absent.

4. Verify the success banner and run link after the POST succeeds.
   **Expected:** The UI reports `Started run ...` and exposes a link to `/runs/$runId`.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| UI control exists after later fixes | Route/component wires `useStartWorkflowRun` | Execute the browser flow through the embedded daemon-served UI |
| Adapter-only support | POST contract exists without UI | File a bug; the case does not pass without a real operator control |
| Hidden/conditional control | Feature-flagged or context-specific action | Execution must document the enabling condition before claiming coverage |

### Related Test Cases

- `TC-FUNC-003`
- `TC-FUNC-007`

### Traceability

- TechSpec API endpoint: `/api/tasks/{slug}/runs`
- Task reference: `task_10`
- Task `15` requirement: run start/cancel coverage must be planned explicitly

### Notes

- `task_16` promoted this case from blocked to executable E2E after wiring the workflow inventory action, switching the browser payload to `presentation_mode: "detach"`, and protecting the path with Playwright plus route/component regression coverage.
