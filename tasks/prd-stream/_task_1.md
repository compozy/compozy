## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/server/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 1.0: SSE Helper Utilities

## Overview

Create reusable SSE helpers for handlers: set headers, write events with id/event/data, flush safely, and emit periodic heartbeats.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Set headers: Content-Type text/event-stream, Cache-Control no-cache, Connection keep-alive, optional X-Accel-Buffering no.
- Parse Last-Event-ID header when present.
- Heartbeat comment frames ': ping' every 15s.
- Keep helpers short; add unit tests.
</requirements>

## Subtasks

- [x] 1.1 Implement StartSSE to set headers and return a flush controller
- [x] 1.2 Implement WriteEvent with flush
- [x] 1.3 Implement WriteHeartbeat and interval helper
- [x] 1.4 Unit tests for headers, event formatting, heartbeats

## Implementation Details

Refer to Tech Spec section “API Implementation Sketch — Shared SSE helper”.

### Relevant Files

- engine/infra/server/router/sse.go
- engine/infra/server/router/sse_test.go

### Dependent Files

- engine/workflow/router/stream.go
- engine/agent/router/stream.go
- engine/task/router/stream.go

## Deliverables

- SSE helper file with exported functions
- Unit tests passing

## Tests

- Unit tests mapped from \_tests.md:
- [x] Headers and flush are set correctly
- [x] Heartbeats emitted without breaking stream

## Success Criteria

- Helpers compiled and covered by tests; no linter issues
- Downstream handlers can stream events with minimal boilerplate
