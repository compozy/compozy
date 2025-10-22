## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/workflow/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal|http_server</dependencies>
</task_context>

# Task 2.0: Workflow Stream Endpoint + Temporal Query Wiring

## Overview

Add GET /executions/workflows/:exec_id/stream. Poll Temporal Query handler (getStreamState) and emit new events via SSE helpers. Support Last-Event-ID and heartbeats; close on completed state.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- SSE endpoint under engine/workflow/router with proper route registration.
- Poll every poll_ms (default 500ms, bounds 250–2000ms).
- Emit only events with id greater than lastEventID.
- Close stream on completed status; log connect/disconnect.
</requirements>

## Subtasks

- [ ] 2.1 Implement handler stream.go using SSE helpers
- [ ] 2.2 Add route in register.go
- [ ] 2.3 Temporal: add SetQueryHandler("getStreamState") in workflow code to return WorkflowStreamState
- [ ] 2.4 Unit tests: cursoring, Last-Event-ID, completion

## Implementation Details

Refer to Tech Spec sections “Workflow layer (Temporal)” and “Workflow stream handler”.

### Relevant Files

- engine/workflow/router/stream.go
- engine/workflow/router/register.go
- engine/worker/... (Temporal workflow query handler)

### Dependent Files

- engine/infra/server/router/sse.go

## Deliverables

- Compiling endpoint, registered route, and passing unit tests

## Tests

- Unit/integration tests mapped from \_tests.md:
  - [ ] Emit only new events since cursor
  - [ ] Honor Last-Event-ID and resume
  - [ ] Close on completed status

## Success Criteria

- Stream provides durable, resumable JSON events for workflows; tests green; no lints
