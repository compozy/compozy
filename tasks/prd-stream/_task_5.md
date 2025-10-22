## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/task/uc</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>redis|http_server</dependencies>
</task_context>

# Task 5.0: Worker Publisher for Text Chunks (Phase 1 shim)

## Overview

Publish simulated per-line text chunks to Redis Pub/Sub channel stream:tokens:<exec_id> when agent/task has no output schema. Use final response content initially; later replace with true provider streaming callbacks.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Identify prompt-only executions with no output schema.
- Split final text into readable lines/chunks and publish sequentially.
- Avoid leaking secrets; reuse redaction utilities.
- Keep publishing out of Temporal workflow determinism paths; use worker task context.
</requirements>

## Subtasks

- [ ] 5.1 Hook into executeAgent path after final output computed
- [ ] 5.2 Implement publisher to Redis Pub/Sub with exec-scoped channel
- [ ] 5.3 Unit/integration tests with miniredis

## Implementation Details

Refer to Tech Spec Phase 3 (worker emission) and Pub/Sub design.

### Relevant Files

- engine/task/uc/exec_task.go
- engine/infra/redis/\*

### Dependent Files

- engine/agent/router/stream.go
- engine/task/router/stream.go

## Deliverables

- Simulated chunk publishing working; tests passing

## Tests

- Tests mapped from \_tests.md:
  - [ ] Publish sequence observed by subscriber in order

## Success Criteria

- Live text observed in /stream consumers; no regressions or leaks
