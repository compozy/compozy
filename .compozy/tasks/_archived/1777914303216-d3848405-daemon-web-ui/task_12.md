---
status: completed
title: Reviews, Spec, and Memory Read Surfaces
type: frontend
complexity: high
dependencies:
  - task_02
  - task_03
  - task_06
  - task_07
---

# Reviews, Spec, and Memory Read Surfaces

## Overview

This task implements the remaining rich-read operator surfaces for v1: reviews, spec, and memory. It completes the mockup-aligned browser console by surfacing review detail and review-fix actions alongside read-only access to PRD, TechSpec, ADR, and memory notebook content.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_02.md` through `task_07.md` before starting
- REFERENCE the TechSpec sections "Route Model", "Active Workspace Model", "Document Read and Cache Strategy", and "Testing Approach"
- FOCUS ON "WHAT" — implement review, spec, and memory inspection plus supported review-fix actions, not browser editing
- MINIMIZE CODE — use the typed client and shared UI layer rather than route-local ad hoc fetch/render logic
- TESTS REQUIRED — review/spec/memory route states and operational review-fix behavior must be covered with executable tests
</critical>

<requirements>
1. MUST implement the review list/detail, spec, and memory routes defined by the TechSpec.
2. MUST keep spec and memory browser surfaces read-only in v1.
3. MUST support review issue detail and the supported review-fix dispatch flow from the browser.
4. MUST consume opaque memory `file_id` values rather than browser-visible file paths.
5. SHOULD include tests for empty, missing, degraded, and stale-workspace states across review/spec/memory flows.
</requirements>

## Subtasks
- [x] 12.1 Build the review, spec, and memory route tree required by the TechSpec.
- [x] 12.2 Render review list/detail surfaces and wire the supported review-fix action flow.
- [x] 12.3 Render spec and memory read surfaces using the typed daemon document payloads.
- [x] 12.4 Ensure opaque memory file IDs and stale-workspace handling are respected in the browser.
- [x] 12.5 Add route, component, and interaction tests for all review/spec/memory states.

## Implementation Details

See the TechSpec sections "Route Model", "Document Read and Cache Strategy", "Data Flow", and "Testing Approach". This task should finish the v1 rich-read browser scope while keeping all authoring capabilities explicitly outside the web UI.

### Relevant Files
- `web/src/routes/_app/reviews.tsx` — review index route for workflow review state.
- `web/src/routes/_app/reviews.$slug.$round.$issueId.tsx` — review issue detail route and review-fix action surface.
- `web/src/routes/_app/workflows.$slug.spec.tsx` — workflow spec read route for PRD, TechSpec, and ADR surfaces.
- `web/src/routes/_app/memory.tsx` — memory index route.
- `web/src/routes/_app/memory.$slug.tsx` — workflow memory detail route using opaque file identifiers.
- `web/src/systems/reviews/`, `web/src/systems/spec/`, `web/src/systems/memory/` — domain logic and rendering for these operator surfaces.

### Dependent Files
- `web/src/storybook/` — later Storybook/MSW work depends on stable review/spec/memory states from this task.
- `web/e2e/` — later Playwright and QA execution depend on the routes and actions implemented here.
- `.compozy/tasks/daemon-web-ui/qa/test-cases/` — later QA planning depends on these read surfaces and review-fix flows being real.
- `web/src/routes/_app.tsx` — shared shell navigation must expose and link into the routes completed here.

### Related ADRs
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — defines the daemon-only read and action contract for these routes.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — makes spec/memory read-only while keeping review-fix actions in scope.

## Deliverables
- Review list/detail routes and supported review-fix browser action flow.
- Read-only spec and memory routes consuming typed daemon document payloads.
- Proper opaque memory-file handling and stale-workspace recovery behavior.
- Unit tests for route/loaders/view models with 80%+ coverage **(REQUIRED)**
- Integration tests for review/spec/memory rendering and review-fix dispatch **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Review, spec, and memory routes load their typed payloads correctly.
  - [x] Memory detail rendering uses opaque file IDs rather than path-derived browser state.
  - [x] Review issue detail and review-fix action state render correctly across loading and error paths.
- Integration tests:
  - [x] Review list and issue detail routes render and dispatch review-fix actions correctly.
  - [x] Spec routes display PRD, TechSpec, and ADR content correctly from daemon responses.
  - [x] Memory index/detail routes render correctly and recover from stale workspace state cleanly.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI exposes all v1 review/spec/memory operator surfaces
- Spec and memory remain read-only while supported review-fix operations work
- Later Storybook, Playwright, and QA tasks can treat these routes as stable product surfaces
