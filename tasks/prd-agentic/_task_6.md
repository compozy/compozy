---
status: completed
parallelizable: false
blocked_by: ["2.0", "3.0", "4.0", "5.0"]
---

<task_context>
<domain>engine/tool/builtin/orchestrate</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"7.0","10.0","11.0"</unblocks>
</task_context>

# Task 6.0: Implement cp\_\_agent_orchestrate builtin handler

## Overview

Create builtin definition with input/output schemas, handler glue to planner/executor, and telemetry hooks using the injected `toolenv.Environment`.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Definition ID `cp__agent_orchestrate`; register input/output schemas through the native provider hook.
- Parse input, compile (prompt → plan) if needed, execute plan using dependencies supplied by `toolenv.Environment`, return structured result.
- Record metrics via existing builtin telemetry helpers without introducing new package cycles.
</requirements>

## Subtasks

- [x] 6.1 Definition and schema wiring
- [x] 6.2 Handler implementation
- [x] 6.3 Tests for happy path and failure cases

## Sequencing

- Blocked by: 2.0, 3.0, 4.0, 5.0
- Unblocks: 7.0, 10.0, 11.0
- Parallelizable: No

## Implementation Details

- Follow existing cp\_\_ tools patterns (fetch/exec/filesystem) for error/metrics, but construct the handler via `NewHandler(env toolenv.Environment, compiler *planner.Compiler, engine *executor.Engine)`.
- Initialize the orchestrator FSM in the handler using the shared helper from Task 5 so planner/executor transitions run under the same `looplab/fsm` contract as `engine/llm/orchestrator`.
- Use the injected environment for agent execution, repository access, and resource lookups; prohibit reliance on context-based globals.
- Thread plan/execution context through every state transition (planner start, validation, execution, finalize, failure) to support telemetry hooks and result aggregation.

### Relevant Files

- `engine/tool/builtin/*`

### Dependent Files

- `engine/tool/native/catalog.go`
- `engine/runtime/toolenv/*`

## Success Criteria

- Tool is discoverable and callable; returns step results
