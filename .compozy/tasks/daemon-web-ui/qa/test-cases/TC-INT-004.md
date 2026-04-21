## TC-INT-004: Run Cancel Action

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `bunx vitest run --config web/vitest.config.ts web/src/routes/-runs.integration.test.tsx web/src/systems/runs/adapters/runs-api.test.ts web/src/systems/runs/components/run-detail-view.test.tsx`
**Automation Notes:** The route suite already proves a running run can be canceled through the daemon contract and that the UI shows the success banner. Adapter and component tests cover transport errors and disabled-button behavior.

### Objective

Verify that the browser can request run cancellation through the daemon API contract and surface the outcome clearly to the operator.

### Preconditions

- [ ] A running run snapshot is available in the test fixture.
- [ ] Run detail route is reachable.
- [ ] Cancellation remains in scope for the run detail surface.

### Test Steps

1. Run the focused cancel-action suite listed above.
   **Expected:** The vitest run exits `0` and the cancel-action specs pass.

2. Confirm the route integration test clicks the cancel control on a running run.
   **Expected:** The route posts to `/api/runs/:run_id/cancel` exactly once.

3. Confirm the success branch.
   **Expected:** The UI renders `run-detail-cancel-success` or equivalent operator feedback.

4. Confirm the error and disabled branches through the component/adaptor tests.
   **Expected:** Transport errors render an error banner, and terminal runs keep the cancel action disabled.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Happy path | Running run, valid POST | Success banner renders |
| Unknown run | `404` from cancel endpoint | Error banner renders |
| Terminal run | Completed/canceled snapshot | Cancel control is disabled |
| Duplicate clicks | Repeated operator input | Only one in-flight cancel request is accepted |

### Related Test Cases

- `TC-FUNC-007`
- `TC-INT-003`

### Traceability

- TechSpec API endpoint: `/api/runs/:run_id/cancel`
- Task reference: `task_10`

### Notes

- Keep cancellation separate from the run-detail smoke lane. It is an operational mutation, not just a visibility check.
