---
status: pending
title: Web App Shell, Workspace Selection, and Dashboard
type: frontend
complexity: high
dependencies:
  - task_02
  - task_03
  - task_06
  - task_07
---

# Web App Shell, Workspace Selection, and Dashboard

## Overview

This task builds the first real browser-facing slice of the daemon web UI: the global shell, workspace selection model, dashboard, and workflow inventory entry route. It establishes the app's route tree and tab-scoped workspace boot behavior that every later frontend slice depends on.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_02.md` through `task_07.md` before starting
- REFERENCE the TechSpec sections "Route Model", "Active Workspace Model", "Data Flow", and "Frontend Module Structure"
- FOCUS ON "WHAT" — build the shell and bootstrap routes first, not every product surface at once
- MINIMIZE CODE — reuse `packages/ui` primitives and the typed API client instead of ad hoc local abstractions
- TESTS REQUIRED — route loaders, workspace selection, and dashboard rendering must be covered with executable tests
</critical>

<requirements>
1. MUST create the TanStack Router shell structure using `__root.tsx`, `_app.tsx`, `_app/index.tsx`, and the workflow inventory entry route.
2. MUST implement the single-workspace-per-tab bootstrap model using `GET /api/workspaces`, `POST /api/workspaces/resolve`, and `sessionStorage`.
3. MUST render the dashboard and workflow inventory using the typed daemon client and the shared UI package.
4. MUST expose supported operational workflow actions that belong on the shell/dashboard surface, including sync and archive where the mockup and TechSpec place them.
5. SHOULD include route and view-model tests covering zero, one, many, and stale-workspace states.
</requirements>

## Subtasks
- [ ] 9.1 Create the root and app-shell route structure aligned with the TechSpec.
- [ ] 9.2 Implement workspace bootstrap, selection, empty state, and stale-workspace recovery.
- [ ] 9.3 Build the dashboard and workflow inventory views using the typed client and shared UI package.
- [ ] 9.4 Add supported workflow-level operational actions exposed from the shell/dashboard surface.
- [ ] 9.5 Add route, loader, and component tests for the shell and dashboard flows.

## Implementation Details

See the TechSpec sections "Route Model", "Active Workspace Model", "Data Flow", and "Frontend Module Structure". This task should establish the app shell and first navigation surfaces so the remaining domain slices can plug into a stable route/layout context.

### Relevant Files
- `web/src/routes/__root.tsx` — root route boundary and app-wide error/not-found handling.
- `web/src/routes/_app.tsx` — global shell route that hosts navigation, selection, and layout state.
- `web/src/routes/_app/index.tsx` — dashboard entry route for the daemon web UI.
- `web/src/routes/_app/workflows.tsx` — workflow inventory route that anchors later workflow-specific navigation.
- `web/src/systems/app-shell/` — shared shell composition and workspace-selection logic for the browser app.
- `web/src/systems/dashboard/` — dashboard-specific data loading and rendering logic.

### Dependent Files
- `web/src/routes/_app/runs.tsx` — later runs work depends on the shell and workspace context established here.
- `web/src/routes/_app/workflows.$slug.tasks.tsx` — later workflow/task routes depend on the shell and workflow inventory navigation.
- `web/src/storybook/` — later route-story coverage depends on stable shell and dashboard states from this task.
- `web/e2e/` — later browser E2E coverage depends on deterministic shell bootstrap and workspace selection behavior.

### Related ADRs
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — defines the route and package structure this task should follow.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — defines the dashboard/workflow shell as an operator-console surface.

## Deliverables
- Root app shell and dashboard/workflow inventory routes.
- Workspace selection and stale-workspace recovery behavior aligned with the TechSpec.
- Dashboard and inventory views using typed client data and shared UI primitives.
- Unit tests for shell/loaders/view models with 80%+ coverage **(REQUIRED)**
- Integration tests for workspace bootstrap and dashboard rendering **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Root and app-shell routes render the expected layout and boundaries.
  - [ ] Workspace bootstrap handles zero, one, many, and stale-workspace cases correctly.
  - [ ] Dashboard and workflow inventory view models consume the typed client data correctly.
- Integration tests:
  - [ ] A fresh browser session can select or resolve an active workspace through the shell.
  - [ ] Dashboard and workflow inventory routes render correctly against real daemon responses.
  - [ ] Supported shell-level sync/archive actions trigger the expected mutations and state refresh behavior.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI has a working shell, dashboard, and workflow inventory entrypoint
- Workspace selection is explicit, tab-scoped, and recoverable
- Later frontend slices can plug into a stable route/layout and workspace context
