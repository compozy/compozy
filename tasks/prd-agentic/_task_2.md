status: completed
parallelizable: false
blocked_by: ["1.0"]
completed_by: ["2.1","2.2","2.3"]
---

<task_context>
<domain>engine/tool/context</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"5.0","6.0"</unblocks>
</task_context>

# Task 2.0: Add tool context bridge for appstate/repo propagation

## Overview

Introduce a typed context bridge (`engine/tool/context`) to expose `*appstate.State`, `task.Repository`, and `resources.ResourceStore` to builtin handlers. Inject these in the execution pipeline before tool invocation.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add getters/setters: `WithAppState`, `GetAppState`, `WithTaskRepo`, `GetTaskRepo`, `WithResourceStore`, `GetResourceStore`.
- Modify `DirectExecutor` execution path (or the nearest safe hook) to attach values to the context used by tool handlers.
- Unit tests for context round‑trip and nil safety.
</requirements>

## Subtasks

- [x] 2.1 Create `engine/tool/context/context.go`
- [x] 2.2 Attach values in `ExecuteTask`/runtime path
- [x] 2.3 Tests for context propagation

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0
- Parallelizable: No (middleware wiring)

## Implementation Details

Relevant sections: Tool context bridge; avoid `context.Background()`; inherit deadlines.

### Relevant Files

- `engine/task/uc/exec_task.go`
- `engine/task/router/direct_executor.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/handler.go`

## Success Criteria

- Builtin handlers can retrieve appstate, repo, and store from context in tests
