---
status: pending
title: Add Multi-Run Daemon API Contracts and Client Surface
type: backend
complexity: medium
dependencies:
  - task_01
---

# Task 2: Add Multi-Run Daemon API Contracts and Client Surface

## Overview

This task adds the daemon transport contract for starting and inspecting a daemon-owned multi-run parent. It establishes typed request, response, handler, client, and OpenAPI surfaces before the run manager implements the coordinator.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- The implementation MUST add a typed request for starting a multi-run parent with workspace, ordered slugs, mode, presentation mode, and runtime overrides.
- The implementation MUST add a typed item/snapshot shape for reconstructing queued, active, completed, failed, and canceled child state.
- The implementation MUST add daemon routes that do not conflict with `/api/tasks/:slug`.
- The implementation MUST add client methods for starting and loading a multi-run parent snapshot.
- The implementation MUST preserve all existing `StartTaskRun` request and route behavior.
- The implementation MUST update contract/OpenAPI tests for the new request schema and route.
</requirements>

## Subtasks

- [ ] 2.1 Add contract types for multi-run start and snapshot payloads.
- [ ] 2.2 Extend the shared API interfaces with multi-run start and snapshot methods.
- [ ] 2.3 Register non-conflicting daemon routes for multi-run start and snapshot reads.
- [ ] 2.4 Add client methods that call the new routes and decode typed responses.
- [ ] 2.5 Add contract, client, handler, and OpenAPI tests for the new API surface.

## Implementation Details

Use `/api/task-runs/multiple` for the start route and `/api/task-runs/multiple/:run_id/snapshot` for the multi-run snapshot route, as described in the TechSpec "API Endpoints" section. Keep child run observation on the existing `/api/runs/:run_id/...` routes.

### Relevant Files

- `internal/api/contract/types.go` — home for transport request and response structs.
- `internal/api/contract/routes.go` — canonical route metadata and timeout classes.
- `internal/api/core/interfaces.go` — shared service interfaces used by HTTP handlers.
- `internal/api/core/routes.go` — route registration for daemon HTTP transport.
- `internal/api/core/handlers.go` — request decoding and service delegation.
- `internal/api/client/client.go` and `internal/api/client/runs.go` — existing client request patterns.
- `internal/api/client/client_contract_test.go` — timeout and request routing expectations.
- `internal/api/httpapi/openapi_contract_test.go` — OpenAPI schema and route contract assertions.

### Dependent Files

- `internal/daemon/task_transport_service.go` — later task will implement the new service methods with `RunManager`.
- `internal/cli/daemon_commands.go` — later task will call the new client methods.
- `internal/core/run/ui/remote.go` — later task will use snapshots to attach the multi-run TUI.

### Related ADRs

- [ADR-004: Use a Daemon-Owned Sequential Multi-Run Coordinator](adrs/adr-004.md) — Requires a parent run API instead of a CLI-only queue.

## Deliverables

- Typed daemon contract for multi-run start and snapshot.
- Registered API routes that avoid `/api/tasks/:slug` conflicts.
- Client methods for multi-run start and snapshot reads.
- Unit tests with 80%+ coverage for request encoding, route timeout class, and response decoding **(REQUIRED)**.
- Handler/OpenAPI integration tests for the new route and schema **(REQUIRED)**.

## Tests

- Unit tests:
  - [ ] Client start method posts to `/api/task-runs/multiple` with ordered slugs.
  - [ ] Client snapshot method gets `/api/task-runs/multiple/<run_id>/snapshot`.
  - [ ] Client rejects a nil context consistently with existing daemon client methods.
  - [ ] Contract route metadata classifies multi-run start as long mutating work.
  - [ ] Contract route metadata classifies multi-run snapshot as read work.
- Integration tests:
  - [ ] Shared handler smoke test starts a multi-run parent through the new route and returns `201`.
  - [ ] OpenAPI contract exposes `TaskRunMultipleRequest` and the new route request body.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing.
- Test coverage >=80%.
- API contracts compile without changing `TaskRunRequest`.
- Existing daemon task-run client and handler tests remain unchanged except for shared interface stubs.
