## TC-FUNC-001: Workspace Bootstrap, Selection, and Stale Recovery

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `bunx vitest run --config web/vitest.config.ts web/src/routes/-app-shell.integration.test.tsx web/src/systems/app-shell/components/app-shell-container.test.tsx web/src/systems/app-shell/hooks/use-active-workspace.test.tsx`
**Automation Notes:** Playwright smoke starts with a resolved workspace from `web/e2e/global.setup.ts`, so the real bootstrap, picker, switch, `sessionStorage`, and stale-`412` branches are currently proved by the route/component integration suites.

### Objective

Verify the TechSpec single-workspace-per-tab model: zero/one/many workspace bootstrap, explicit selection, switch-workspace behavior, and stale-workspace recovery.

### Preconditions

- [ ] Bun dependencies are installed.
- [ ] Generated route tree and typed client are current.
- [ ] The active-workspace store is reset before execution.
- [ ] Browser session state is cleared or the tests handle their own cleanup.

### Test Steps

1. Run the focused workspace-bootstrap suite listed above.
   **Expected:** The vitest run exits `0` and all listed specs pass.

2. Confirm the suite covers the zero-workspace onboarding branch.
   **Expected:** The shell renders `workspace-onboarding` when `/api/workspaces` returns an empty list.

3. Confirm the suite covers explicit workspace selection when multiple workspaces exist.
   **Expected:** Selecting a workspace persists `compozy.web.active-workspace`, loads the dashboard, and updates the active-workspace banner.

4. Confirm the suite covers stale-workspace recovery.
   **Expected:** A stale selected workspace is cleared from store and `sessionStorage`, and the picker reopens with a stale-state message instead of leaving the shell broken.

5. Confirm the switch-workspace path remains wired through the shared shell container.
   **Expected:** The shell can return to the picker and select a different workspace without page corruption.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Zero workspaces | `GET /api/workspaces -> []` | Onboarding/picker flow is shown |
| One workspace | Single registered workspace | Dashboard loads automatically |
| Many workspaces | Two or more workspaces | Explicit selection is required |
| Stale workspace | `sessionStorage` contains removed ID | Selection is cleared and picker is shown |

### Related Test Cases

- `TC-FUNC-002`
- `TC-INT-002`

### Traceability

- TechSpec: "Active Workspace Model"
- Task references: `task_09`, `task_14`, `task_15`
- Shared memory note: browser workspace header and stale recovery are normalized through the HTTP browser layer

### Notes

- Treat this as the first executable blocker. The rest of the browser matrix depends on a trustworthy workspace context.
