---
status: completed
title: Workflow Task Board and Task Detail Surfaces
type: frontend
complexity: high
dependencies:
  - task_02
  - task_03
  - task_06
  - task_07
---

# Workflow Task Board and Task Detail Surfaces

## Overview

This task implements the workflow-oriented task board and task detail views described by the daemon mockup and TechSpec. It gives the browser operator a structured view of task status, dependencies, and related runtime context without crossing into in-browser authoring.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_02.md` through `task_07.md` before starting
- REFERENCE the TechSpec sections "Route Model", "Backend Read Models", and "Data Flow"
- FOCUS ON "WHAT" — implement workflow/task inspection and navigation, not task editing
- MINIMIZE CODE — use shared shell/UI primitives and the typed client rather than task-local abstractions
- TESTS REQUIRED — board, task-detail, and workflow-navigation behavior must be covered with executable tests
</critical>

<requirements>
1. MUST implement the workflow task surfaces at `/workflows/$slug/tasks` and `/workflows/$slug/tasks/$taskId`.
2. MUST render the task board/list, task detail metadata, and related run context from the daemon read models.
3. MUST keep these surfaces read-oriented and MUST NOT introduce browser task authoring or editing flows.
4. MUST integrate with the workflow navigation and shell context established by the earlier app-shell task.
5. SHOULD include tests covering empty workflow boards, stale task IDs, and related-run display states.
</requirements>

## Subtasks
- [x] 11.1 Build the workflow task-board route and the task-detail route structure.
- [x] 11.2 Render the board/list view using the daemon's task board payload.
- [x] 11.3 Render task detail metadata, dependency context, and related run information.
- [x] 11.4 Integrate workflow/task navigation cleanly into the shared shell and workspace context.
- [x] 11.5 Add route and component tests covering success, empty, and invalid task states.

## Implementation Details

See the TechSpec sections "Route Model", "Backend Read Models", "Data Flow", and "Testing Approach". This task should establish the task-centric workflow inspection surfaces while keeping editing capabilities explicitly out of scope for v1.

### Relevant Files
- `web/src/routes/_app/workflows.$slug.tasks.tsx` — route for workflow task board/list rendering.
- `web/src/routes/_app/workflows.$slug.tasks.$taskId.tsx` — route for task detail rendering.
- `web/src/systems/workflows/` — workflow- and task-centric domain logic for loaders, view models, and composition.
- `web/src/systems/app-shell/` — shell/navigation context that the workflow/task surfaces must integrate with.
- `internal/api/core/interfaces.go` — task board and task detail payload reference for the browser UI.
- `internal/daemon/task_transport_service.go` — backend transport source for task/workflow detail semantics used by this UI.

### Dependent Files
- `web/src/storybook/` — later route-state stories depend on stable workflow/task surfaces from this task.
- `web/e2e/` — later browser E2E coverage depends on deterministic board and task-detail flows.
- `.compozy/tasks/daemon-web-ui/qa/test-cases/` — later QA planning depends on the workflow/task flows implemented here.
- `web/src/routes/_app/workflows.tsx` — workflow inventory navigation should link coherently into these routes.

### Related ADRs
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — defines the route/system structure used here.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — keeps task surfaces inspectable but not editable.

## Deliverables
- Workflow task-board and task-detail routes in the daemon web UI.
- Board/list/detail rendering backed by typed daemon payloads.
- Shell-integrated workflow/task navigation without browser editing features.
- Unit tests for route/loaders/view models with 80%+ coverage **(REQUIRED)**
- Integration tests for workflow board and task-detail rendering **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Task-board and task-detail routes load the expected payloads through the typed client.
  - [x] Workflow/task view models render status, dependencies, and related-run data correctly.
  - [x] Invalid or stale task identifiers surface clear not-found/error states.
- Integration tests:
  - [x] `/workflows/$slug/tasks` renders the daemon task board correctly.
  - [x] `/workflows/$slug/tasks/$taskId` renders detail and related-run context correctly.
  - [x] Workflow/task navigation works correctly from the shell and workflow inventory surface.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI can inspect workflow task status and detail without authoring features
- Workflow/task navigation is coherent inside the shared shell
- Later Storybook, E2E, and QA work can treat task surfaces as stable operator views
