---
status: pending
parallelizable: true
blocked_by: ["1.0", "2.0"]
---

<task_context>
<domain>engine/agent/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>worker,http_server,idempotency,uc.ExecuteTask</dependencies>
<unblocks>6.0, 7.0, 8.0, 9.0</unblocks>
</task_context>

# Task 3.0: Implement direct Agent execution endpoints (sync/async)

## Overview

Expose direct execution for agents:

- Sync: `POST /api/v0/agents/{agent_id}/executions`
- Async: `POST /api/v0/agents/{agent_id}/executions/async`

Validate input (`action` or `prompt` required). Sync enforces `timeout` (default 60s, max 300s); async ignores `timeout` and returns `202` with `Location`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Define request/response DTOs per Tech Spec.
- Use API idempotency helper; dedupe on route+body or header.
- For sync: execute via `uc.NewExecuteTask(...).Run(...)`, persist state, respect timeout, return `{ output, exec_id }`.
- For async: enqueue/start execution and return `{ exec_id, exec_url }` with `Location: /api/v0/executions/agents/{exec_id}`.
- Use `router.NewRequestError` for 400/404/409/503/500; never use `context.Background()`.
- Read limits/timeouts from `config.FromContext(ctx)`; log via `logger.FromContext(ctx)`.
</requirements>

## Subtasks

- [ ] 3.1 Define DTOs and validation (action|prompt, with, timeout)
- [ ] 3.2 Implement sync path with server-side wait and timeout enforcement
- [ ] 3.3 Implement async path, status URL, and `Location` header
- [ ] 3.4 Unit tests for validation, success, idempotency, and timeout

## Sequencing

- Blocked by: 1.0, 2.0
- Unblocks: 6.0, 7.0, 8.0, 9.0
- Parallelizable: Yes (with 4.0 after foundations)

## Implementation Details

Map errors per Tech Spec. Ensure authZ via middleware and worker readiness checks. Persist task state before/after execution for status lookups.

### Relevant Files

- `engine/agent/router/exec.go`
- `engine/infra/server/router/idempotency.go`
- `engine/infra/server/router/helpers.go`
- `engine/task/uc/exec_task.go`

### Dependent Files

- `engine/worker/manager.go`
- `engine/task/repo.go`

## Success Criteria

- Endpoints behave per PRD (R8–R16)
- Proper idempotency semantics and headers
- Lints/tests pass
