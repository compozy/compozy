---
status: completed
parallelizable: false
blocked_by: ["1.0", "2.0", "4.0", "6.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>response_handler</dependencies>
<unblocks>8.0</unblocks>
</task_context>

# Task 7.0: Implement `handle_completion` (retry/success) mapping

## Overview

Handle completion attempts without tools in `enter_handle_completion`, emitting `completion_retry` or `completion_success` per validator outcomes; preserve structured output semantics.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Mirror current validator and retry logic
- Ensure consistent metadata packaging for downstream `finalize`
- Log outcomes and reasons
</requirements>

## Subtasks

- [x] 7.1 Map handler results to events
- [x] 7.2 Tests for retry/success paths and edge cases

## Sequencing

- Blocked by: 1.0, 2.0, 4.0, 6.0
- Unblocks: 8.0
- Parallelizable: No

## Implementation Details

Refer to `response_handler.go` tests to keep behavior parity.

### Relevant Files

- `engine/llm/orchestrator/response_handler.go`
- `engine/llm/orchestrator/state_machine.go`

### Dependent Files

- `engine/llm/orchestrator/memory.go`

## Success Criteria

- Tests cover both outcomes; lints/tests pass
