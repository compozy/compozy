## TC-INT-001: Review-Fix Dispatch

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `bunx vitest run --config web/vitest.config.ts web/src/routes/-reviews-flow.integration.test.tsx web/src/systems/reviews/adapters/reviews-api.test.ts`
**Automation Notes:** The review detail route already proves dispatch through the typed browser contract, including the workspace-scoped POST body and the success link into the newly created run.

### Objective

Verify that the supported browser review-fix action dispatches a run through the daemon contract and returns usable operator feedback.

### Preconditions

- [ ] Review detail payloads are available through the typed browser client.
- [ ] The active workspace is already selected.
- [ ] Review-fix action remains in scope for the current route.

### Test Steps

1. Run the focused review-fix suite listed above.
   **Expected:** The vitest run exits `0` and the dispatch-related specs pass.

2. Confirm the route integration test clicks `review-detail-dispatch-fix`.
   **Expected:** The browser flow posts to `/api/reviews/:slug/rounds/:round/runs`.

3. Confirm the success state includes a run deep link.
   **Expected:** The UI exposes the dispatched run ID and links to `/runs/$runId`.

4. Confirm the request threads the active workspace into the daemon contract.
   **Expected:** The POST body includes the selected workspace identifier rather than relying on ambient browser state.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Successful dispatch | Valid review issue and round | Success banner/link renders |
| Transport error | Conflict or backend error | Operator-visible error banner renders |
| Invalid round | Non-numeric route round | Dispatch path is not attempted |

### Related Test Cases

- `TC-FUNC-005`
- `TC-FUNC-007`

### Traceability

- TechSpec API endpoint: `/api/reviews/:slug/rounds/:round/runs`
- Task reference: `task_12`
- ADR-004: review-fix is one of the supported v1 operational actions

### Notes

- Keep this in the targeted suite even if review navigation is already green in smoke. The dispatch mutation is a separate operator contract.
