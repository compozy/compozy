---
status: pending
parallelizable: true
blocked_by: ["1.0", "2.0"]
---

<task_context>
<domain>engine/task/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>worker,http_server,idempotency,uc.ExecuteTask</dependencies>
<unblocks>6.0, 7.0, 8.0, 9.0</unblocks>
</task_context>

# Task 4.0: Implement direct Task execution endpoints (sync/async)

## Overview

Expose direct execution for tasks:

- Sync: `POST /api/v0/tasks/{task_id}/executions`
- Async: `POST /api/v0/tasks/{task_id}/executions/async`

Validate body; sync enforces `timeout` (default 60s, max 300s); async ignores `timeout` and returns `202` + `Location`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Mirror Agent exec semantics for request/response and idempotency.
- For sync: call `uc.NewExecuteTask` and persist state; return `{ output, exec_id }`.
- For async: return `{ exec_id, exec_url }` with `Location: /api/v0/executions/tasks/{exec_id}`.
- Use `router.NewRequestError`; never use `context.Background()`.
- Read limits/timeouts from `config.FromContext(ctx)`; log via `logger.FromContext(ctx)`.
</requirements>

## Subtasks

- [ ] 4.1 DTOs and validation (with, timeout)
- [ ] 4.2 Implement sync path with wait/timeout
- [ ] 4.3 Implement async path and status URL
- [ ] 4.4 Unit tests (validation, idempotency, outcomes)

## Sequencing

- Blocked by: 1.0, 2.0
- Unblocks: 6.0, 7.0, 8.0, 9.0
- Parallelizable: Yes (with 3.0)

## Implementation Details

Follow Tech Spec mapping R17–R24. Ensure authZ checks and consistent error mapping.

### Relevant Files

- `engine/task/router/exec.go`
- `engine/infra/server/router/idempotency.go`
- `engine/infra/server/router/helpers.go`
- `engine/task/uc/exec_task.go`

### Dependent Files

- `engine/worker/manager.go`
- `engine/task/repo.go`

## Success Criteria

- Endpoints behave per PRD (R17–R24)
- Idempotency and response envelopes match standards
- Lints/tests pass
