---
status: pending
title: Additive Browser REST Endpoints and OpenAPI Wiring
type: backend
complexity: high
dependencies:
  - task_02
  - task_05
---

# Additive Browser REST Endpoints and OpenAPI Wiring

## Overview

This task exposes the new browser-oriented daemon reads and actions through additive REST endpoints while preserving the current root-scoped API families. It also aligns the shared handler stack and the checked-in OpenAPI contract so the browser-facing API surface is explicit and testable.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, `task_02.md`, and `task_05.md` before starting
- REFERENCE the TechSpec sections "API Endpoints", "OpenAPI Generation Contract", and "Impact Analysis"
- FOCUS ON "WHAT" — add the required browser routes and handler contracts without replacing existing daemon APIs
- MINIMIZE CODE — keep route registration and handler logic thin by delegating to transport services
- TESTS REQUIRED — route, handler, and contract alignment must be covered with executable tests
</critical>

<requirements>
1. MUST add the browser-oriented read routes and action surfaces described in the TechSpec, including dashboard, spec, memory, board, task detail, and review issue detail endpoints.
2. MUST preserve the existing root-scoped route families such as `/api/tasks`, `/api/reviews`, `/api/runs`, `/api/sync`, and `/api/workspaces`.
3. MUST keep non-success responses in the shared daemon problem-envelope format, including the web-specific `412` workspace-context failure.
4. MUST align the checked-in OpenAPI artifact with the actual handler and route behavior introduced here.
5. SHOULD add handler tests for success, empty, not-found, conflict, and stale-workspace cases where applicable.
</requirements>

## Subtasks
- [ ] 6.1 Extend the shared route registration with the additive browser endpoints required by the TechSpec.
- [ ] 6.2 Add or extend handler methods and DTOs so the new browser routes are fully implemented.
- [ ] 6.3 Keep the existing root-scoped daemon routes intact and compatible while adding the new surfaces.
- [ ] 6.4 Update the OpenAPI artifact so it matches the real browser-facing route and payload behavior.
- [ ] 6.5 Add handler and contract tests for the new web routes and problem responses.

## Implementation Details

See the TechSpec sections "API Endpoints", "OpenAPI Generation Contract", "Impact Analysis", and "Testing Approach". This task should finalize the browser-facing REST surface while preserving transport parity and the current daemon route families.

### Relevant Files
- `internal/api/core/routes.go` — shared route registration point for the daemon API families.
- `internal/api/core/interfaces.go` — shared DTO and service interface layer that the new browser routes must extend.
- `internal/api/core/handlers.go` — shared handler implementations that must surface the new browser reads and actions.
- `internal/api/core/errors.go` — shared problem-envelope behavior, including web-specific status handling.
- `internal/api/core/handlers_test.go` — existing handler test surface that should grow with the new browser endpoints.
- `openapi/compozy-daemon.json` — checked-in browser contract that must match the real route/handler surface.

### Dependent Files
- `internal/api/core/middleware.go` — later workspace/security middleware depends on the new routes being present.
- `web/src/lib/api-client.ts` — typed browser client depends on the paths and payloads stabilized here.
- `web/src/routes/` — later route/domain tasks depend on these endpoints being real and typed.
- `internal/api/httpapi/routes.go` — shared HTTP registration depends on the new core route set.

### Related ADRs
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — requires the browser to consume a daemon-owned contract.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — defines the scope of new reads and operational actions.

## Deliverables
- Additive browser endpoints implemented in the shared daemon route/handler layer.
- Updated shared DTOs and problem-envelope handling for the browser surface.
- Aligned checked-in OpenAPI artifact covering the implemented routes.
- Unit tests for routes/handlers/problem responses with 80%+ coverage **(REQUIRED)**
- Integration tests proving the additive routes work without regressing the existing daemon API families **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] New browser routes register under the expected root-scoped API families without removing existing routes.
  - [ ] Handlers return the expected payloads and problem envelopes for success and failure paths.
  - [ ] Stale or missing workspace context returns the expected typed problem response.
- Integration tests:
  - [ ] Existing `/api/tasks`, `/api/reviews`, `/api/runs`, `/api/sync`, and `/api/workspaces` flows still behave correctly after the new routes are added.
  - [ ] The additive browser routes return payloads compatible with the checked-in OpenAPI document.
  - [ ] Shared HTTP/UDS route registration stays coherent after the browser routes are introduced.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon exposes every browser-facing REST endpoint required by the TechSpec
- Existing root-scoped daemon APIs remain intact and compatible
- The OpenAPI artifact and the real handler surface no longer drift
