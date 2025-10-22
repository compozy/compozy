## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/task/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database|redis</dependencies>
</task_context>

# Task 4.0: Task Stream Endpoint (structured + text)

## Overview

Add GET /executions/tasks/:exec_id/stream mirroring agent behavior. Structured JSON when output schema exists; text-only llm_chunk via Pub/Sub otherwise. Close on terminal state.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Same branching rules as agent endpoint.
- Structured: repo polling + JSON events.
- Text: Pub/Sub subscription + llm_chunk forwarding.
- Heartbeats and headers via SSE helpers.
</requirements>

## Subtasks

- [ ] 4.1 Implement task stream handler and route
- [ ] 4.2 Structured repo polling
- [ ] 4.3 Pub/Sub subscribe path
- [ ] 4.4 Unit/integration tests (miniredis)

## Implementation Details

Follow agent stream design; share helpers where possible.

### Relevant Files

- engine/task/router/stream.go
- engine/task/router/register.go
- engine/infra/redis/\*

### Dependent Files

- engine/infra/server/router/sse.go
- engine/task/repo.go

## Deliverables

- Task stream endpoint with tests passing

## Tests

- Tests mapped from \_tests.md:
  - [ ] Structured: JSON deltas and completion
  - [ ] Text: llm_chunk forwarding via miniredis

## Success Criteria

- Endpoint stable and mirrors agent behavior; tests green
