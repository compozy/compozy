---
status: completed
title: Shared UI Package and Mockup Theme Foundations
type: frontend
complexity: high
dependencies:
  - task_01
---

# Shared UI Package and Mockup Theme Foundations

## Overview

This task creates the shared design-system layer for the daemon web UI inside `packages/ui/`. It adapts the AGH-style UI package boundary to the local daemon mockup by defining tokens, typography, shadcn primitives, and app-shell building blocks before the route-level product work begins.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `docs/design/daemon-mockup/colors_and_type.css` before starting
- REFERENCE the TechSpec sections "Theme and visual system", "Frontend Module Structure", and "Development Sequencing"
- FOCUS ON "WHAT" — establish reusable UI and theme foundations, not product route logic
- MINIMIZE CODE — prefer shared primitives and tokens over route-local one-off styling
- TESTS REQUIRED — component, token, and package-export behavior must be covered with executable tests
</critical>

<requirements>
1. MUST create `packages/ui/` as the shared home for reusable primitives, tokens, typography, and shell-level composition helpers.
2. MUST derive the initial theme tokens and font strategy from `docs/design/daemon-mockup/colors_and_type.css`, including defined fallbacks where licensing or packaging requires them.
3. MUST align with the AGH runtime structure and shadcn/Tailwind conventions without copying AGH product visuals.
4. MUST expose a stable package export surface that `web/` can consume for shell/layout and common UI building blocks.
5. SHOULD include component-level tests proving the package renders and exports correctly under the chosen theme foundation.
</requirements>

## Subtasks
- [x] 3.1 Create the `packages/ui/` source, export, and build structure expected by the daemon SPA.
- [x] 3.2 Port the initial token, typography, and font-loading strategy from the daemon mockup into reusable CSS/theme assets.
- [x] 3.3 Add the first shared primitives and shell helpers needed by the app shell and domain routes.
- [x] 3.4 Wire the package exports and local consumption path from `web/`.
- [x] 3.5 Add tests proving token loading, package exports, and primitive rendering behavior.

## Implementation Details

See the TechSpec sections "Theme and visual system", "Frontend Module Structure", and "Testing Approach". This task should establish the reusable visual and component layer so later route/domain tasks do not need to duplicate primitives or invent their own token system.

### Relevant Files
- `docs/design/daemon-mockup/colors_and_type.css` — source of truth for the mockup's color and type direction.
- `packages/ui/package.json` — shared UI package manifest and export surface.
- `packages/ui/src/index.ts` — primary package export barrel for reusable UI elements.
- `packages/ui/src/tokens.css` — shared token/theme entrypoint expected by later app-shell work.
- `packages/ui/src/lib/utils.ts` — shared utility surface expected by shadcn-style primitives and AGH-like structure.
- `packages/ui/vitest.config.ts` — package-local test configuration expected by later verification work.
- `web/components.json` — shadcn/components registry contract that should point at the shared UI boundary.
- `/Users/pedronauck/dev/compozy/agh/packages/ui/package.json` — closest structural reference for the AGH-style UI package boundary.

### Dependent Files
- `web/src/routes/_app.tsx` — later app shell work depends on the shared shell and navigation primitives from this task.
- `web/src/components/` — route-level composition should consume the package exports established here.
- `web/src/storybook/` — later Storybook/MSW work depends on stable UI package exports and tokens.
- `web/package.json` — later route and test tooling depends on `@.../ui` workspace consumption being stable.

### Related ADRs
- [ADR-001: Mirror AGH's Runtime Frontend Topology with `web/` and `packages/ui`](adrs/adr-001.md) — establishes the package boundary this task must honor.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — keeps the shared UI package focused on operator-console surfaces, not editor tooling.

## Deliverables
- Shared `packages/ui/` package with stable exports for tokens, typography, primitives, and shell helpers.
- Mockup-aligned theme/token foundation with explicit font fallback behavior.
- Initial shadcn/Tailwind-aligned reusable UI components for later route work.
- Unit tests for token/package/component behavior with 80%+ coverage **(REQUIRED)**
- Integration checks proving `web/` can consume the shared package cleanly **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Token and typography assets load with the expected package export paths.
  - [x] Shared primitives render correctly under the daemon theme foundation.
  - [x] Package exports resolve without leaking route-specific implementation details.
- Integration tests:
  - [x] `web/` can import and render the shared UI package successfully.
  - [x] Shared token CSS is applied consistently across package and app consumers.
  - [x] Theme and primitive changes do not require route-level style duplication.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `packages/ui/` is the shared source for daemon web UI primitives and tokens
- The daemon mockup's visual language is represented in reusable theme assets
- Later route tasks can build on shared UI exports instead of local one-off components
