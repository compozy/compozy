## TC-INT-005: Mocked Route-State Harness Parity

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `bunx vitest run --config web/vitest.config.ts web/src/storybook/route-stories.test.tsx`
**Automation Notes:** This portable-story suite verifies that loading, empty, degraded, partial, and error branches across the major routes still render inside the Storybook/MSW harness introduced by `task_13`.

### Objective

Verify that the mocked review harness remains aligned with the real route tree so execution can inspect important non-happy-path states without rebuilding fixtures during `task_16`.

### Preconditions

- [ ] Storybook route stories exist under `web/src/routes/_app/stories/`.
- [ ] MSW handlers remain colocated with the corresponding systems.
- [ ] The portable-story test still runs through the web Vitest configuration.

### Test Steps

1. Run the portable route-story suite listed above.
   **Expected:** The vitest run exits `0` and the route-story cases pass.

2. Confirm degraded/loading dashboard states are covered.
   **Expected:** Dashboard degraded and loading stories render with the expected test IDs.

3. Confirm empty/error task, workflow, run, review, spec, and memory states are covered.
   **Expected:** The suite renders representative empty/error/partial branches for each route family.

4. Confirm the route stories still match the actual route/system structure.
   **Expected:** Story imports and handlers resolve without drift from the app code.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Dashboard degraded | Portable story | `dashboard-view` degraded branch renders |
| Workflow error | Portable story | Workflow inventory error branch renders |
| Run overflow | Portable story | Run detail overflow branch renders |
| Memory document error | Portable story | Memory detail error branch renders |

### Related Test Cases

- `TC-FUNC-006`
- `TC-INT-002`

### Traceability

- ADR-005: Storybook/MSW route coverage is part of the core verification bar
- Task reference: `task_13`

### Notes

- This is the route-state backstop for branches that are hard to seed through the live daemon harness alone.
