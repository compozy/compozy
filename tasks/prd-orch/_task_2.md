---
status: completed
parallelizable: false
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/llm/orchestrator</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>state_machine, orchestrator entrypoints</dependencies>
<unblocks>3.0, 4.0, 5.0, 9.0, 10.0</unblocks>
</task_context>

# Task 2.0: Replace legacy loop with FSM in conversationLoop.Run

## Overview

Refactor `conversationLoop.Run` to initialize the FSM, trigger `start_loop`, and drive execution with `fsm.Event(...)` until a terminal state, removing the manual `for` loop and scattered branching.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Initialize `LoopContext` and `*fsm.FSM` via `newLoopFSM`
- Remove the manual iteration loop and route control through events
- Maintain external behavior and return types; no feature flags
- Use `logger.FromContext(ctx)` for lifecycle logs
</requirements>

## Subtasks

- [x] 2.1 Replace `for` loop with FSM event driving
- [x] 2.2 Ensure outputs bubble to callers (no API changes)
- [ ] 2.3 Update unit tests in `orchestrator_execute_test.go` accordingly

## Sequencing

- Blocked by: 1.0
- Unblocks: 3.0, 4.0, 5.0, 9.0, 10.0
- Parallelizable: No (core refactor)

## Implementation Details

See Tech Spec: “FSM-driven loop” and sample builder. Keep structured outputs and retry semantics unchanged.

### Relevant Files

- `engine/llm/orchestrator/loop.go`
- `engine/llm/orchestrator/orchestrator.go`

### Dependent Files

- `engine/llm/orchestrator/llm_invoker.go`
- `engine/llm/orchestrator/response_handler.go`

## Success Criteria

- FSM drives control flow; tests pass with behavior parity
- Linters green
