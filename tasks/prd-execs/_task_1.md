---
status: pending
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/agent/router,engine/task/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server,database</dependencies>
<unblocks>3.0, 4.0</unblocks>
</task_context>

# Task 1.0: Add execution status endpoints for agents and tasks

## Overview

Add read-only status endpoints to fetch the state of direct executions:

- `GET /api/v0/executions/agents/{exec_id}`
- `GET /api/v0/executions/tasks/{exec_id}`

Return terminal and in-progress states using the standard `router.Response` envelope. These endpoints are used by async flows.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Register routes under agent and task routers.
- Use `router.GetAgentExecID(c)` and `router.GetTaskExecID(c)` to parse IDs.
- Load persisted task state via repository/use cases and map to DTO.
- Enforce authN/Z via existing middleware; return 404 when not found.
- Use `router.NewRequestError` for error mapping; never leak internal errors.
- Use `logger.FromContext(ctx)` for structured logs; avoid globals.
</requirements>

## Subtasks

- [ ] 1.1 Add route definitions and handlers
- [ ] 1.2 Integrate repository to fetch execution state
- [ ] 1.3 Unit tests: 200 (found), 404 (missing), error mapping

## Sequencing

- Blocked by: —
- Unblocks: 3.0, 4.0
- Parallelizable: No (foundation for async flows)

## Implementation Details

See Tech Spec sections “System Architecture → Domain Placement” and “Implementation Design → Core Interfaces”. Response payload should include `exec_id`, `status`, and result if terminal.

### Relevant Files

- `engine/agent/router/exec.go`
- `engine/task/router/exec.go`

### Dependent Files

- `engine/infra/server/router/helpers.go`
- `engine/task/repo.go`

## Success Criteria

- Endpoints return correct status and payload shape
- Conformant error responses and logs
- Lints/tests pass
