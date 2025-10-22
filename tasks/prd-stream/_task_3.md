## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/agent/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server|database|redis</dependencies>
</task_context>

# Task 3.0: Agent Stream Endpoint (structured + text)

## Overview

Add GET /executions/agents/:exec_id/stream. If output schema exists, emit structured JSON updates by polling repo. If no schema, forward live text chunks from Redis Pub/Sub channel stream:tokens:<exec_id> as llm_chunk events. Close on terminal state.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Branch by schema presence (agent action or task output schema).
- Structured: poll task repo, emit JSON deltas, include completion event.
- Text-only: subscribe to Pub/Sub and forward per-line llm_chunk events.
- Respect heartbeats and headers via SSE helpers.
</requirements>

## Subtasks

- [x] 3.1 Implement agent stream handler and route
- [x] 3.2 Repo polling path for structured JSON
- [x] 3.3 Pub/Sub subscribe path for text chunks; graceful shutdown
- [x] 3.4 Unit/integration tests (miniredis for Pub/Sub)

## Implementation Details

See Tech Spec Option A branching rules and Pub/Sub design.

### Relevant Files

- engine/agent/router/stream.go
- engine/agent/router/register.go
- engine/infra/redis/\*

### Dependent Files

- engine/infra/server/router/sse.go
- engine/task/repo.go

## Deliverables

- Agent stream endpoint with structured and text branches, tests passing

## Tests

- Tests mapped from \_tests.md:
- [x] Structured: JSON deltas and completion
- [x] Text: llm_chunk forwarding via miniredis

## Success Criteria

- Endpoint stable under reconnects; correct behavior in both modes
