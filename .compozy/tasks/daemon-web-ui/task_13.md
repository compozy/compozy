---
status: completed
title: Storybook, MSW, and Mocked Route-State Coverage
type: frontend
complexity: high
dependencies:
  - task_03
  - task_09
  - task_10
  - task_11
  - task_12
---

# Storybook, MSW, and Mocked Route-State Coverage

## Overview

This task adds the full mocked review surface the user asked for: Storybook across routes and components, backed by MSW handlers and route-state helpers. It turns the daemon web UI into something that can be reviewed and regression-checked visually and interactively without depending on live daemon state for every iteration.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_03.md`, `task_09.md` through `task_12.md` before starting
- REFERENCE the TechSpec sections "Testing Approach", "Frontend Module Structure", and "Development Sequencing"
- FOCUS ON "WHAT" — create the mocked review harness for components and routes, not a second production app
- MINIMIZE CODE — colocate stories and MSW fixtures near the domains they exercise
- TESTS REQUIRED — Storybook configuration, MSW contracts, and representative route/component states must be covered with executable checks
</critical>

<requirements>
1. MUST add Storybook support for both `web/` routes and `packages/ui/` reusable components.
2. MUST use MSW to mock the daemon routes and component states required by the user and the TechSpec.
3. MUST cover the major route-state surfaces for dashboard, workflows, runs, reviews, spec, and memory, including loading, empty, success, degraded, and error states.
4. MUST keep mocked route stories and fixtures colocated with the route/domain systems they exercise where practical.
5. SHOULD add configuration and contract checks proving Storybook and MSW wiring stays in sync with the app structure.
</requirements>

## Subtasks
- [x] 13.1 Add Storybook configuration for `web/` and `packages/ui/`.
- [x] 13.2 Add MSW setup and route-state helpers for mocked daemon responses.
- [x] 13.3 Create route stories for dashboard, workflows, runs, reviews, spec, and memory states.
- [x] 13.4 Create or expand shared component stories in `packages/ui/` for reusable primitives and shell pieces.
- [x] 13.5 Add configuration and contract tests covering Storybook/MSW behavior.

## Implementation Details

See the TechSpec sections "Testing Approach", "Frontend Module Structure", and "Development Sequencing". This task should deliver the full mocked route/component review layer required by ADR-005 and requested explicitly by the user.

### Relevant Files
- `web/src/storybook/msw.ts` — MSW bootstrap and handler registry pattern used by the AGH web app.
- `web/src/storybook/route-story.tsx` — route-story helper pattern for mocked route rendering.
- `web/src/routes/_app/stories/` — route-state story location pattern used by AGH and appropriate for daemon-web-ui routes.
- `packages/ui/.storybook/` — package-level Storybook configuration for the shared UI package.
- `packages/ui/src/components/stories/` — shared component story location for the reusable UI package.
- `web/src/routes/` — route modules whose loading/empty/success/error states must gain mocked stories.
- `/Users/pedronauck/dev/compozy/agh/web/src/storybook/` — closest reference for Storybook/MSW structure in the AGH runtime app.

### Dependent Files
- `web/playwright.config.ts` — later E2E work can reuse stable route-state fixtures and assumptions from this task.
- `.compozy/tasks/daemon-web-ui/qa/test-cases/` — later QA planning depends on a stable matrix of route states and mocked evidence.
- `web/src/systems/` — domain components depend on mocked stories to stay reviewable as they evolve.
- `packages/ui/package.json` — package-level Storybook scripts and tests depend on the configuration introduced here.

### Related ADRs
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — defines the structural split for route and component stories.
- [ADR-005: Require Full Frontend Verification with Vitest, Playwright, Storybook, and MSW](adrs/adr-005.md) — makes Storybook/MSW part of the primary scope.

## Deliverables
- Storybook configuration for `web/` and `packages/ui/`.
- MSW handlers and route-state helpers for mocked daemon route responses.
- Route stories for dashboard, workflows, runs, reviews, spec, and memory states.
- Unit tests for Storybook/MSW configuration with 80%+ coverage **(REQUIRED)**
- Integration checks proving stories render representative route/component states successfully **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Storybook configuration resolves stories and shared package imports correctly.
  - [x] MSW handlers and route-state helpers provide the expected mocked daemon responses.
  - [x] Representative route stories cover loading, empty, success, degraded, and error states.
- Integration tests:
  - [x] Route stories render correctly for dashboard, workflows, runs, reviews, spec, and memory.
  - [x] Shared component stories render cleanly from `packages/ui/`.
  - [x] Storybook/MSW wiring remains aligned with the actual route and component structure.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon web UI has a complete Storybook/MSW review harness for routes and components
- Mocked route states are easy to inspect without requiring live daemon state
- Later Playwright and QA work can build on a stable mocked-state surface
