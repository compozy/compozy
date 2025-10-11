---
status: completed
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/agent/exec</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
<unblocks>"2.0","3.0","5.0","6.0"</unblocks>
</task_context>

# Task 1.0: Extract reusable Agent Runner service from router

## Overview

Create a reusable `Runner` service that encapsulates synchronous agent execution logic currently embedded in `engine/agent/router/exec.go`, delegating to `tkrouter.DirectExecutor`. This enables internal callers (builtins) to execute agents without duplicating HTTP-layer code.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Provide `NewRunner(state *appstate.State, repo task.Repository, store resources.ResourceStore)` constructor.
- Implement `Execute(ctx, ExecuteRequest) (*ExecuteResult, error)` that loads agent config, validates action/prompt, builds a transient `task.Config`, and calls `DirectExecutor.ExecuteSync`.
- Preserve idempotency and timeout semantics equivalent to agent sync endpoints.
- Add unit tests covering happy path, unknown agent/action, timeout, and executor error.
</requirements>

## Subtasks

- [x] 1.1 Define `engine/agent/exec/runner.go` with public API
- [x] 1.2 Migrate helper logic from router to service (no behavior change)
- [x] 1.3 Update router to use `Runner` (thin wrapper), add tests

## Sequencing

- Blocked by: None
- Unblocks: 2.0, 3.0, 5.0, 6.0
- Parallelizable: No (foundational service)

## Implementation Details

Relevant sections from tech spec: Runner extraction; reuse DirectExecutor; single orchestration path.

### Relevant Files

- `engine/agent/router/exec.go`
- `engine/task/router/direct_executor.go`

### Dependent Files

- `engine/tool/builtin/orchestrate/*` (later tasks)

## Success Criteria

- Router endpoints pass all tests and delegate to Runner
- New unit tests for Runner pass; behavior parity preserved
