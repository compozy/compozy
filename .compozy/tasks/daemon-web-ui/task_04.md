---
status: pending
title: Daemon Projection and Document Query Layer
type: backend
complexity: high
dependencies: []
---

# Daemon Projection and Document Query Layer

## Overview

This task builds the internal read-model layer for the daemon web UI before any browser-facing handlers are added. It is responsible for assembling dashboard, spec, memory, task detail, review detail, and run-related projections from `global.db`, `run.db`, and canonical workspace documents without leaking filesystem assumptions into the browser.

<critical>
- ALWAYS READ `_techspec.md` and ADRs before starting; there is no `_prd.md` for this feature
- REFERENCE the TechSpec sections "Daemon read-model services", "Backend Read Models", and "Document Read and Cache Strategy"
- FOCUS ON "WHAT" — assemble the query/data layer behind the UI, not the HTTP contract itself
- MINIMIZE CODE — keep browser concerns and route rendering out of this layer
- TESTS REQUIRED — read-model and document assembly behavior must be covered with executable tests
</critical>

<requirements>
1. MUST provide internal query surfaces for dashboard, workflow overview, spec, memory, task detail, review detail, and run-related read models required by the TechSpec.
2. MUST read canonical PRD, TechSpec, ADR, and memory documents server-side and normalize them into typed payloads suitable for later transport mapping.
3. MUST use opaque memory file identifiers rather than browser-visible relative paths.
4. MUST keep read-model logic outside HTTP handlers and browser middleware.
5. SHOULD structure the query layer so later cache invalidation and watcher integration can be added without rewriting the payload model.
</requirements>

## Subtasks
- [ ] 4.1 Define the internal read models needed by dashboard, spec, memory, task detail, and review detail views.
- [ ] 4.2 Implement server-side document readers for spec and memory artifacts with normalized payload shapes.
- [ ] 4.3 Assemble workflow, task, review, and run projections from the existing daemon databases.
- [ ] 4.4 Add opaque memory file identity handling suitable for later transport exposure.
- [ ] 4.5 Add tests covering query assembly, document normalization, and path-hiding behavior.

## Implementation Details

See the TechSpec sections "Daemon read-model services", "Backend Read Models", "Source of Truth Rules", and "Document Read and Cache Strategy". This task should create the payload assembly layer that later transport services can consume directly, without encoding HTTP-specific behavior or browser headers.

### Relevant Files
- `internal/store/globaldb/` — existing persisted workflow, run, review, and registry projections that the web UI must read.
- `internal/daemon/run_snapshot.go` — current run snapshot assembly path that informs the run-detail read model.
- `internal/core/tasks/` — existing task artifact parsing logic relevant to task and workflow metadata reads.
- `internal/core/reviews/` — existing review artifact handling relevant to round and issue projections.
- `internal/daemon/watchers.go` — watcher surface that later cache invalidation can attach to.
- `internal/api/core/interfaces.go` — current DTO vocabulary that the new internal read models must align with before transport exposure.

### Dependent Files
- `internal/daemon/task_transport_service.go` — later transport service work depends on stable task/workflow/read-model assembly from this task.
- `internal/daemon/review_exec_transport_service.go` — later review and issue transport reads depend on the projections built here.
- `internal/api/core/handlers.go` — later browser endpoints depend on the normalized payloads defined here.
- `openapi/compozy-daemon.json` — later contract wiring depends on the shapes stabilized by this read-model layer.

### Related ADRs
- [ADR-003: Use Daemon-Only REST/SSE Contracts with OpenAPI-Generated Web Types](adrs/adr-003.md) — requires the daemon to be the only source for browser reads.
- [ADR-004: Scope V1 to Operational and Rich Read Surfaces, Not In-Browser Authoring](adrs/adr-004.md) — constrains these read models to operational and read-only artifact surfaces.

## Deliverables
- Internal read models for dashboard, task detail, review detail, spec, memory, and related run data.
- Server-side document readers that normalize workspace artifacts for browser consumption.
- Opaque memory file identity handling suitable for later transport exposure.
- Unit tests for query and document behavior with 80%+ coverage **(REQUIRED)**
- Integration tests for DB-plus-filesystem projection assembly **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] Dashboard projections assemble daemon, workflow, and run summary data into the expected internal shape.
  - [ ] Spec and memory document readers normalize markdown artifacts without exposing raw filesystem paths.
  - [ ] Memory index generation returns stable opaque file IDs rather than browser-visible relative paths.
- Integration tests:
  - [ ] Workflow/task/review/run data can be assembled from real daemon database fixtures.
  - [ ] Missing or stale workspace documents return typed failures suitable for later problem-envelope translation.
  - [ ] Run-related read models compose cleanly with persisted daemon snapshots and event data.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- The daemon has an internal query layer for every new web read surface in scope
- Browser-facing handlers can consume normalized read models instead of ad hoc filesystem access
- Document and memory reads are prepared for opaque ID transport and later invalidation work
