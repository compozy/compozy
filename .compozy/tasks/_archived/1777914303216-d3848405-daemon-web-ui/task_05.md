---
status: completed
title: Daemon Web Read-Model Transport Services
type: backend
complexity: high
dependencies:
  - task_04
---

# Daemon Web Read-Model Transport Services

## Overview

This task wires the new internal read-model/query layer into the daemon transport services used by the shared HTTP and UDS handler stack. It keeps the DB-plus-filesystem composition in transport services and mappers rather than pushing that complexity into the handlers themselves.

<critical>
- ALWAYS READ `_techspec.md`, ADRs, and `task_04.md` before starting
- REFERENCE the TechSpec sections "Daemon read-model services", "Core Interfaces", and "Impact Analysis"
- FOCUS ON "WHAT" — expose the new read models through transport services, not through route-level handler logic
- MINIMIZE CODE — keep joins and mapping logic centralized in transport services and helpers
- TESTS REQUIRED — transport service behavior must be proven with daemon-level tests
</critical>

<requirements>
1. MUST extend the daemon transport service layer so the new web read models are available through shared service interfaces.
2. MUST keep the DB-plus-filesystem joins inside daemon transport services or mapper helpers instead of HTTP handlers.
3. MUST replace existing unsupported transport stubs where the web UI depends on those surfaces.
4. MUST preserve compatibility for current CLI/UDS consumers while adding the new browser-facing reads.
5. SHOULD add explicit mapper/service tests for dashboard, task detail, review detail, spec, and memory transport shapes.
</requirements>

## Subtasks
- [x] 5.1 Extend or add transport services that expose the new read-model surfaces required by the web UI.
- [x] 5.2 Centralize transport mapping logic for the new dashboard, detail, spec, and memory payloads.
- [x] 5.3 Remove or replace unsupported transport stubs relied on by the new browser flows.
- [x] 5.4 Keep current daemon runtime consumers compatible while adding the new read surfaces.
- [x] 5.5 Add service and mapper tests covering success and failure paths.

## Implementation Details

See the TechSpec sections "Daemon read-model services", "Core Interfaces", and "Impact Analysis". This task should be the seam between the internal query layer and the shared transport/handler layer, so later HTTP work can stay thin and contract-focused.

### Relevant Files
- `internal/daemon/task_transport_service.go` — current task transport service with unsupported read surfaces that the web UI needs.
- `internal/daemon/review_exec_transport_service.go` — current review transport path that must grow richer read/detail support.
- `internal/daemon/workspace_transport_service.go` — workspace transport surface that later active-workspace flows depend on.
- `internal/daemon/sync_transport_service.go` — existing sync transport path that must stay compatible with the web UI's operational model.
- `internal/daemon/transport_mappers.go` — natural place to centralize transport-facing mapping for new read models.
- `internal/daemon/run_manager.go` — operational run surfaces that the transport layer must integrate with for run-related reads/actions.

### Dependent Files
- `internal/api/core/interfaces.go` — later handler interface expansion depends on the services stabilized here.
- `internal/api/core/handlers.go` — later browser endpoints should call these services rather than reassembling data.
- `internal/api/core/routes.go` — new browser routes depend on stable service methods and payloads.
- `internal/api/client/` — later client compatibility work depends on transport behavior remaining coherent.

### Related ADRs
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — requires daemon-owned transport surfaces for browser reads.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — constrains transport work to operational and read surfaces.

## Deliverables
- Extended daemon transport services exposing the web UI read surfaces.
- Centralized transport mappers for the new dashboard/detail/spec/memory payloads.
- Removal or replacement of unsupported transport stubs required by the browser flows.
- Unit tests for transport mapping/service behavior with 80%+ coverage **(REQUIRED)**
- Integration tests proving transport services can serve real daemon data **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Transport services return the expected dashboard, detail, spec, and memory payload shapes.
  - [x] Unsupported paths that the browser depends on are replaced with real service behavior.
  - [x] Mapper helpers preserve IDs, timestamps, and status data correctly across the new payloads.
- Integration tests:
  - [x] Shared transport services can assemble the new payloads from real daemon DB/document fixtures.
  - [x] Failure paths from missing workspace, workflow, review, or document data are surfaced as transport-level errors.
  - [x] Existing operational run surfaces remain compatible while the new reads are added.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon transport layer can serve every new web read surface without handler-side joins
- Existing CLI/UDS consumers are not broken by the richer transport model
- Later HTTP route work can stay thin and contract-driven
