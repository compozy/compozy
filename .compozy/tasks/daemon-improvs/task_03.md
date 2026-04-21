---
status: pending
title: Shared Transport Contract Migration
type: refactor
complexity: critical
dependencies:
  - task_01
  - task_02
---

# Shared Transport Contract Migration

## Overview

This task migrates the shared daemon transport layer to the canonical contract without replacing the current service split. It removes inline and anonymous handler-owned response shapes, aligns HTTP and UDS behavior, and locks parity on the full route inventory the daemon exposes today.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Component Overview", "API Endpoints", "Error-Code Vocabulary", and "Development Sequencing" instead of duplicating them here
- FOCUS ON "WHAT" - migrate transport ownership to the canonical contract while preserving existing service boundaries
- MINIMIZE CODE - reuse shared handler logic and avoid transport-specific DTO forks between HTTP and UDS
- TESTS REQUIRED - every changed endpoint and stream surface needs parity-oriented test coverage
</critical>

<requirements>
1. MUST migrate `internal/api/core`, `internal/api/httpapi`, and `internal/api/udsapi` to use `internal/api/contract` for JSON payloads, SSE records, and transport errors.
2. MUST preserve the current `TaskService`, `ReviewService`, `RunService`, `WorkspaceService`, `SyncService`, and `ExecService` seams rather than introducing a second transport facade.
3. MUST eliminate anonymous handler response shapes where a canonical envelope now exists.
4. MUST make HTTP and UDS return the same status codes, envelope fields, request IDs, and SSE event names across the full route inventory in `internal/api/core/routes.go`.
5. SHOULD keep transport-specific code limited to listener bootstrap, middleware wiring, and shutdown behavior rather than payload ownership.
</requirements>

## Subtasks

- [ ] 3.1 Rewire core handlers and shared transport helpers to consume the canonical contract package.
- [ ] 3.2 Remove or replace inline DTOs and anonymous response bodies that now belong to the canonical contract.
- [ ] 3.3 Align HTTP and UDS transports so route registration, error rendering, and SSE semantics stay in parity.
- [ ] 3.4 Update handler and transport tests to assert canonical envelopes and full route coverage.
- [ ] 3.5 Verify that parity failures are caught through the shared integration harness before downstream client migration begins.

## Implementation Details

Implement the transport migration described in the TechSpec sections "Component Overview", "API Endpoints", "Impact Analysis", and "Build Order". This task should move the transport layer onto the canonical contract while preserving the existing transport-facing service split for daemon-owned business logic.

### Relevant Files

- `internal/api/core/handlers.go` - shared route handlers currently own JSON shaping and must switch to canonical contract types.
- `internal/api/core/interfaces.go` - inline transport shapes that remain after task 01 must be removed or reduced to service-facing contracts only.
- `internal/api/core/errors.go` - transport problem rendering must emit the frozen canonical error vocabulary.
- `internal/api/core/sse.go` - SSE emission must use canonical `event`, `heartbeat`, and `overflow` records.
- `internal/api/httpapi/server.go` - HTTP transport must render canonical responses without transport-local DTO drift.
- `internal/api/udsapi/server.go` - UDS transport must mirror the same contract semantics as HTTP.
- `internal/api/httpapi/transport_integration_test.go` - transport parity assertions should be upgraded to the shared canonical envelopes.

### Dependent Files

- `internal/api/httpapi/routes.go` - route registration should remain unchanged while using the canonical handler outputs.
- `internal/api/udsapi/routes.go` - route registration must stay in lockstep with HTTP while using the same contract.
- `internal/daemon/task_transport_service.go` - task-start responses will be consumed through the canonical transport envelopes.
- `internal/daemon/review_exec_transport_service.go` - review and exec transport surfaces depend on the migrated handler contract.
- `internal/daemon/workspace_transport_service.go` - workspace registration and resolution responses must now flow through the shared contract.
- `internal/daemon/sync_transport_service.go` - sync responses and warnings must be rendered through canonical envelopes.

### Related ADRs

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) - requires the transport layer to converge on the canonical contract package.
- [ADR-003: Validation-First Daemon Hardening](adrs/adr-003.md) - requires HTTP and UDS parity coverage for the migrated surfaces.

## Deliverables

- Shared handlers, HTTP transport, and UDS transport migrated to canonical contract-owned payloads and SSE shapes.
- Removal of remaining anonymous transport response bodies where canonical envelopes exist.
- Expanded parity coverage for daemon, workspace, task, review, run, sync, and exec routes.
- Unit tests with 80%+ coverage for handler error rendering, SSE framing, and canonical envelope use **(REQUIRED)**
- Integration tests proving HTTP and UDS parity for representative endpoints and stream surfaces **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] `GET /api/daemon/health` returns the same canonical health envelope and degraded details whether readiness is true or false.
  - [ ] `POST /api/tasks/:slug/runs` and `POST /api/reviews/:slug/rounds/:round/runs` emit canonical run envelopes instead of handler-local anonymous payloads.
  - [ ] `GET /api/runs/:run_id/stream` emits canonical `event`, `heartbeat`, and `overflow` frame names and payload fields.
  - [ ] Transport errors for validation, not found, and conflict paths emit canonical code and request ID fields.
- Integration tests:
  - [ ] HTTP and UDS both return the same envelope shape and status code for `GET /api/workspaces`, `POST /api/exec`, and `GET /api/runs/:run_id/snapshot`.
  - [ ] HTTP and UDS both emit equivalent SSE streams for one run, including heartbeat and overflow behavior under the same harness scenario.
  - [ ] Route coverage includes at least one endpoint from daemon, workspaces, tasks, reviews, runs, sync, and exec groups.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- HTTP and UDS emit one canonical contract across the full shared route inventory
- Handler-local anonymous payload shapes are removed where canonical envelopes exist
- Existing daemon service interfaces remain intact while transport ownership moves to `internal/api/contract`
