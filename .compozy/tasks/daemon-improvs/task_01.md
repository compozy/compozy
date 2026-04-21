---
status: pending
title: Canonical Daemon Contract Foundation
type: refactor
complexity: high
dependencies: []
---

# Canonical Daemon Contract Foundation

## Overview

This task establishes `internal/api/contract` as the canonical daemon transport contract described by the TechSpec. It freezes the JSON, SSE, cursor, timeout, and error semantics that every later migration task depends on, while keeping the current service split transport-neutral.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Component Overview", "Core Interfaces", "Data Models", and "API Endpoints" instead of duplicating them here
- FOCUS ON "WHAT" - extract and freeze the contract boundary before migrating handlers, clients, or runtime logic
- MINIMIZE CODE - move transport shapes into one package and avoid inventing parallel runtime abstractions
- TESTS REQUIRED - unit and integration coverage are mandatory for all contract semantics
</critical>

<requirements>
1. MUST create `internal/api/contract` as the single source of truth for daemon DTOs, response envelopes, transport error payloads, SSE record types, and cursor formatting/parsing.
2. MUST cover the full route inventory currently registered in `internal/api/core/routes.go`, including daemon, workspace, task, review, run, sync, and exec surfaces.
3. MUST freeze the canonical error-code vocabulary, timeout class names, and stream heartbeat/overflow semantics so later tasks do not redefine them ad hoc.
4. MUST preserve current `RunSnapshot` compatibility expectations for downstream readers and document any adapter requirement instead of introducing a silent shape change.
5. SHOULD keep `TaskService`, `ReviewService`, `ExecService`, and `RunService` transport-neutral by moving only JSON-facing contract ownership into the new package.
</requirements>

## Subtasks

- [ ] 1.1 Extract transport-facing DTOs, envelopes, and cursor types from `internal/api/core` into `internal/api/contract`.
- [ ] 1.2 Define canonical SSE frame, heartbeat, overflow, and cursor semantics in the new contract package.
- [ ] 1.3 Freeze the route inventory, transport error vocabulary, and timeout class taxonomy in contract-owned types and helpers.
- [ ] 1.4 Add compatibility notes for `RunSnapshot`, `RunEventPage`, and run-reader-facing payloads so later migrations know what must remain stable.
- [ ] 1.5 Add contract-focused tests that lock JSON serialization, cursor parsing, and error-envelope behavior before transport migration begins.

## Implementation Details

Implement the boundary described in the TechSpec sections "Component Overview", "Core Interfaces", "Data Models", "Error-Code Vocabulary", and "Timeout Policy". This task should stop at defining and testing the canonical contract; it should not yet migrate all handlers or client call sites to use it end-to-end.

### Relevant Files

- `internal/api/core/interfaces.go` - current home of transport DTOs and run snapshot shapes that must move behind the canonical contract.
- `internal/api/core/errors.go` - current transport problem helpers and error envelope logic that must align with frozen codes.
- `internal/api/core/sse.go` - current SSE framing, cursor, heartbeat, and overflow semantics to standardize.
- `internal/api/core/routes.go` - authoritative route inventory that the contract package must fully cover.
- `internal/api/contract/types.go` - new canonical DTO and envelope definitions for the daemon API.
- `internal/api/contract/errors.go` - new typed error-code vocabulary and transport error helpers.
- `internal/api/contract/sse.go` - new canonical stream cursor and SSE frame helpers.

### Dependent Files

- `internal/api/core/handlers.go` - later handler migration must consume the canonical contract instead of inline response shapes.
- `internal/api/client/client.go` - later client work depends on the timeout taxonomy and error envelope definitions frozen here.
- `internal/api/client/runs.go` - run snapshot and event paging client code depends on stable cursor and page contracts.
- `pkg/compozy/runs/run.go` - public run-reader compatibility depends on the snapshot and stream shapes defined here.
- `internal/api/httpapi/server.go` - HTTP transport parity depends on the contract package rather than transport-local structs.
- `internal/api/udsapi/server.go` - UDS transport parity depends on the same canonical contract definitions.

### Related ADRs

- [ADR-001: Canonical Daemon Transport Contract](adrs/adr-001.md) - establishes `internal/api/contract` as the canonical daemon boundary.
- [ADR-004: Observability as a First-Class Daemon Contract](adrs/adr-004.md) - requires snapshot and observability payloads to be part of the same contract surface.

## Deliverables

- New `internal/api/contract` package with canonical DTO, envelope, SSE, cursor, and error helpers.
- Frozen route inventory and timeout taxonomy captured in contract-facing definitions.
- Compatibility notes for `RunSnapshot` and public run-reader surfaces.
- Unit tests with 80%+ coverage for serialization, cursor parsing, and error-envelope behavior **(REQUIRED)**
- Integration tests proving contract fixtures decode consistently through current transport helpers **(REQUIRED)**

## Tests

- Unit tests:
  - [ ] Serializing and deserializing a daemon status, health, run, snapshot, and event-page payload round-trips without dropping required fields.
  - [ ] Parsing and formatting a stream cursor with timestamp and sequence preserves ordering semantics across repeated round-trips.
  - [ ] Emitting a transport error with `daemon_not_ready`, `conflict`, and `schema_too_new` returns the expected code, message, and request ID fields.
  - [ ] Encoding heartbeat and overflow SSE payloads preserves the canonical field names and timestamp semantics defined by the contract.
- Integration tests:
  - [ ] Existing handler and SSE tests can consume the new contract types without changing route semantics for `GET /api/daemon/health` and `GET /api/runs/:run_id/stream`.
  - [ ] Snapshot payloads generated from current fixtures still decode into the canonical `RunSnapshot` shape without losing `jobs`, `usage`, or `shutdown`.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- `internal/api/contract` becomes the only owned source of daemon transport shapes
- Route inventory, error codes, and timeout classes are frozen in one canonical boundary
- Snapshot and stream compatibility rules are explicit before downstream migration starts
