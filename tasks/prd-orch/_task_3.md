---
status: completed
parallelizable: true
blocked_by: ["1.0", "2.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>llm_invoker</dependencies>
<unblocks>6.0, 9.0</unblocks>
</task_context>

# Task 3.0: Implement `await_llm` state and `llm_response` event

## Overview

Invoke the LLM in `enter_await_llm`, pass the response via the `llm_response` event to `evaluate_response`, and log iteration context via `logger.FromContext(ctx)`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Trigger LLM call inside `enter_await_llm`
- Attach response to event metadata for downstream states
- Ensure context propagation and structured logs
</requirements>

## Subtasks

- [x] 3.1 Wire `enter_await_llm` to `llm_invoker`
- [x] 3.2 Emit `llm_response` with payload
- [x] 3.3 Unit tests for event emission and logging

## Sequencing

- Blocked by: 1.0, 2.0
- Unblocks: 6.0, 9.0
- Parallelizable: Yes (post 1.0/2.0)

## Implementation Details

Ensure parity with current request assembly and timing logs from the legacy loop.

### Relevant Files

- `engine/llm/orchestrator/llm_invoker.go`
- `engine/llm/orchestrator/state_machine.go`

### Dependent Files

- `engine/llm/orchestrator/evaluate_response.go` (branching; see Task 4.0)

## Success Criteria

- Tests validate event flow and logging; lints/tests pass
