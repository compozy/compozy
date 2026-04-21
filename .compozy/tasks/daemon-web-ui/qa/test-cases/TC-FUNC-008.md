## TC-FUNC-008: Workflow Run Start from the Browser Surface

**Priority:** P1 (High)
**Type:** Functional
**Status:** Blocked
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Blocked
**Automation Status:** Gap
**Automation Command/Spec:**
- Inspection command: `rg -n "useStartWorkflowRun|startWorkflowRun" web/src`
- Supporting seam: `bunx vitest run --config web/vitest.config.ts web/src/systems/runs/adapters/runs-api.test.ts`
**Automation Notes:** The browser POST contract for `/api/tasks/{slug}/runs` exists in the adapter and hook layer, but no route/component wiring or visible control was found in `web/src/routes` or `web/src/systems` for a real operator to trigger it in the delivered UI.

### Objective

Determine whether a real browser operator can start a workflow run from the current daemon web UI, and if not, preserve that absence as an explicit blocker rather than silently omitting the flow.

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

4. If no browser entrypoint exists at execution time, create a bug/blocker artifact instead of marking the flow passed.
   **Expected:** The verification report records this case as blocked with concrete evidence.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| UI control exists after later fixes | Route/component wires `useStartWorkflowRun` | Reclassify this case to `E2E` or `Integration` during `task_16` and execute it |
| Adapter-only support | POST contract exists without UI | Case remains blocked and becomes an issue candidate |
| Hidden/conditional control | Feature-flagged or context-specific action | Execution must document the enabling condition before claiming coverage |

### Related Test Cases

- `TC-FUNC-003`
- `TC-FUNC-007`

### Traceability

- TechSpec API endpoint: `/api/tasks/{slug}/runs`
- Task reference: `task_10`
- Task `15` requirement: run start/cancel coverage must be planned explicitly

### Notes

- This is a blocker case, not a manual-only case. The current evidence says the contract exists but the browser entrypoint does not.
