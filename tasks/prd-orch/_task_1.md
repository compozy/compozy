---
status: completed
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>looplab/fsm, configuration</dependencies>
<unblocks>2.0, 3.0, 4.0, 5.0, 9.0</unblocks>
</task_context>

# Task 1.0: Create FSM scaffolding and builder (state_machine.go)

## Overview

Introduce an explicit FSM for the orchestrator with well-defined states/events and a builder function `newLoopFSM`. Define state and event identifiers, minimal `LoopContext`, and callback hooks to host side-effects while preserving external behavior.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Provide `engine/llm/orchestrator/state_machine.go` with:
  - `const` state IDs: init, await_llm, evaluate_response, process_tools, update_budgets, handle_completion, finalize, terminate_error
  - `const` event IDs: start_loop, llm_response, response_no_tool, response_with_tools, tools_executed, budget_ok, budget_exceeded, completion_retry, completion_success, failure
  - `func newLoopFSM(ctx context.Context, deps loopDeps, loopCtx *LoopContext) *fsm.FSM`
- No global singletons. Always pass context; use `logger.FromContext(ctx)`.
- Read configuration only via `config.FromContext(ctx)` if needed.
</requirements>

## Subtasks

- [x] 1.1 Define state and event identifiers
- [x] 1.2 Implement `newLoopFSM` with event table and empty callbacks
- [x] 1.3 Add unit test verifying allowed transitions and initial state

## Sequencing

- Blocked by: —
- Unblocks: 2.0, 3.0, 4.0, 5.0, 9.0
- Parallelizable: No (foundation)

## Implementation Details

Use the Tech Spec sections “Proposed States” and “Transition Events”. Ensure the FSM is context-aware and thread-safe per library guarantees.

### Relevant Files

- `engine/llm/orchestrator/state_machine.go`

### Dependent Files

- `engine/llm/orchestrator/loop.go`
- `engine/llm/orchestrator/orchestrator.go`

## Success Criteria

- Builder compiles, tests cover event table and initial state
- Lints/tests pass
