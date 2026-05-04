## TC-FUNC-002: Dashboard Load and Workflow Inventory Navigation

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 10 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `bun run frontend:e2e`
- Primary spec: `web/e2e/daemon-ui.smoke.spec.ts`
**Automation Notes:** The Playwright smoke harness boots the real daemon, seeds the fixture workspace, serves the embedded SPA from the daemon HTTP listener, and verifies the dashboard shell plus workflow inventory navigation.

### Objective

Verify that a real browser can load the embedded dashboard, resolve the active workspace banner, and navigate from `/` into the workflow inventory through the daemon-served UI.

### Preconditions

- [ ] `bin/compozy` is built and current.
- [ ] Playwright global setup can seed the fixture workspace and start the daemon.
- [ ] The repo is ready to serve the embedded SPA rather than a Vite-only dev server.

### Test Steps

1. Run the Playwright smoke suite listed above.
   **Expected:** The suite exits `0` and the daemon-served browser session starts successfully.

2. Open the root route `/`.
   **Expected:** `dashboard-view` is visible and the active workspace banner matches the seeded workspace name.

3. Trigger the workflow inventory navigation from the dashboard.
   **Expected:** The workflow inventory route renders and shows workflow cards from the seeded fixture.

4. Confirm the flow is using the daemon-served runtime, not Vite dev mode.
   **Expected:** The base URL is the ephemeral daemon HTTP listener created by Playwright setup, and deep links resolve under the same origin.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Fresh seeded workspace | Default Playwright fixture | Dashboard loads with active workspace banner |
| Inventory navigation | Click dashboard "view all workflows" action | Workflow inventory route becomes visible |
| Embedded serving | Daemon HTTP listener | Same origin serves `/` and `/api` |

### Related Test Cases

- `TC-FUNC-001`
- `TC-FUNC-003`

### Traceability

- TechSpec: "Route Model", "Data Flow"
- ADR-002: embedded SPA from the daemon HTTP listener
- ADR-005: browser coverage is part of the core verification bar
- Task references: `task_09`, `task_14`

### Notes

- This case proves the core browser entrypoint exists and is daemon-served. It does not replace the workspace bootstrap checks in `TC-FUNC-001`.
