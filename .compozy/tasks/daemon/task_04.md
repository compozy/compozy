---
status: completed
title: Shared Transport Core
type: backend
complexity: critical
dependencies:
  - task_01
  - task_02
  - task_03
---

# Shared Transport Core

## Overview
This task creates the shared daemon transport layer over UDS and localhost HTTP, including request IDs, JSON error envelopes, health/metrics routes, and the canonical SSE stream contract. It is the control-plane foundation that later CLI, TUI, reviews, exec, and public run-reader tasks will all consume.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "API Endpoints", "Transport Contract", and "SSE Contract" instead of duplicating them here
- FOCUS ON "WHAT" — shared semantics between UDS and HTTP matter more than any one server implementation detail
- MINIMIZE CODE — keep one handler core and avoid route drift between transports
- TESTS REQUIRED — unit and integration coverage are mandatory for route parity and streaming contracts
</critical>

<requirements>
1. MUST expose shared handler logic over both UDS and localhost HTTP with no behavioral drift between transports.
2. MUST emit `X-Request-Id` on all responses and serialize non-2xx responses through the `TransportError` envelope.
3. MUST implement the SSE cursor contract defined in the TechSpec, including `RFC3339Nano|sequence`, `Last-Event-ID`, heartbeat, and overflow semantics.
4. MUST expose daemon status, health, metrics, snapshot, event pagination, and stream routes needed by later CLI and TUI work.
5. MUST bind HTTP only to `127.0.0.1`, use an ephemeral port persisted in daemon state, and create the UDS socket with `0600`.
</requirements>

## Subtasks
- [x] 4.1 Create the shared transport interfaces and handler core used by both UDS and HTTP.
- [x] 4.2 Register daemon, workspace, task, review, run, sync, and exec routes with one resource model.
- [x] 4.3 Add request-ID propagation and the uniform JSON error envelope.
- [x] 4.4 Implement the SSE stream contract, including cursor resume, heartbeat, and overflow behavior.
- [x] 4.5 Add parity tests for UDS, HTTP, health, metrics, and streaming behavior.

## Implementation Details
Implement the transport layer described in the TechSpec "API Endpoints", "Transport Contract", "SSE Contract", and "Monitoring and Observability" sections. This task should establish the external contract only; it should rely on injected services instead of embedding run logic or storage details directly in transport code.

### AGH Reference Files
- `~/dev/compozy/agh/internal/api/core/interfaces.go` — reference for transport-neutral handler/service boundaries.
- `~/dev/compozy/agh/internal/api/core/handlers.go` — reference for shared handler implementation patterns.
- `~/dev/compozy/agh/internal/api/core/sse.go` — reference for cursor, heartbeat, and overflow stream behavior.
- `~/dev/compozy/agh/internal/api/httpapi/server.go` — reference for localhost HTTP server wiring.
- `~/dev/compozy/agh/internal/api/httpapi/routes.go` — reference for route registration and parity.
- `~/dev/compozy/agh/internal/api/udsapi/server.go` — reference for UDS bootstrap and socket-permission handling.
- `~/dev/compozy/agh/internal/api/udsapi/routes.go` — reference for UDS route parity with HTTP.

### Relevant Files
- `internal/api/core/interfaces.go` — new shared transport service contracts for handlers and clients.
- `internal/api/core/handlers.go` — new shared handler implementations for daemon resources.
- `internal/api/core/sse.go` — new SSE writer and cursor logic aligned with AGH semantics.
- `internal/api/httpapi/server.go` — new localhost HTTP server bootstrap and middleware wiring.
- `internal/api/httpapi/routes.go` — new HTTP route registration for daemon resources.
- `internal/api/udsapi/server.go` — new UDS server bootstrap with socket permission handling.
- `internal/api/udsapi/routes.go` — new UDS route registration with parity to HTTP.

### Dependent Files
- `internal/cli/state.go` — later daemon client work will depend on request IDs, route shapes, and conflict semantics defined here.
- `internal/core/run/ui/model.go` — remote attach will depend on snapshot and stream behavior introduced here.
- `pkg/compozy/runs/watch.go` — public run-reader migration will depend on the SSE and snapshot contract from this task.
- `internal/core/extension/runtime.go` — later extension adaptation will depend on the shared transport and error semantics.

### Related ADRs
- [ADR-001: Adopt a Global Home-Scoped Singleton Daemon](adrs/adr-001.md) — requires stable local transport surfaces for the singleton daemon.
- [ADR-003: Expose the Daemon Through AGH-Aligned REST Transports Using Gin](adrs/adr-003.md) — defines the transport shape and reuse target.

## Deliverables
- Shared handler core for daemon resources.
- UDS and localhost HTTP server implementations with aligned route registration.
- Request-ID propagation, JSON error envelopes, and SSE cursor/heartbeat/overflow support.
- Unit tests with 80%+ coverage for handler logic and SSE helpers **(REQUIRED)**
- Integration tests covering UDS/HTTP parity, request IDs, health, metrics, and stream resume **(REQUIRED)**

## Tests
- Unit tests:
  - [x] Shared handlers return the same status and payload shape for the same service result across both transports.
  - [x] SSE helpers emit `id`, `event`, and `data` frames using the `RFC3339Nano|sequence` cursor format.
  - [x] Invalid `Last-Event-ID` values are rejected or normalized deterministically according to the daemon stream contract.
  - [x] Heartbeat and overflow frames are emitted with the expected event names and payload shape.
  - [x] Non-2xx responses include `X-Request-Id` and serialize the `TransportError` envelope correctly.
- Integration tests:
  - [x] `GET /daemon/health` returns `503` before readiness and `200` once the daemon is fully ready.
  - [x] `GET /runs/:run_id/stream` resumes from the next event after `Last-Event-ID` and emits heartbeats during idle periods.
  - [x] `GET /runs/:run_id/stream` returns the expected validation response for an invalid or stale cursor.
  - [x] UDS and localhost HTTP serve matching behavior for status, snapshot, and conflict responses.
  - [x] Status, metrics, and stream endpoints remain observable and reconnectable when a client disconnects or when the daemon closes a stream at terminal run state.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- UDS and localhost HTTP expose one aligned daemon API contract
- Request IDs and error envelopes are consistent across all responses
- SSE resume, heartbeat, and overflow behavior are stable enough for later attach/watch clients
