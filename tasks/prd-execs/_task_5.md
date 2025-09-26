---
status: pending
parallelizable: false
blocked_by: ["2.0"]
---

<task_context>
<domain>engine/workflow/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>temporal,workflow_repo,http_server,idempotency</dependencies>
<unblocks>6.0, 7.0, 8.0, 9.0</unblocks>
</task_context>

# Task 5.0: Implement synchronous Workflow execution endpoint

## Overview

Add `POST /api/v0/workflows/{workflow_id}/executions/sync` that triggers a workflow and waits (bounded) for completion using repo polling. Enforce `timeout` default 60s and max 300s; return 200 on success or 408 on server-side timeout.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `engine/workflow/router/execute_sync.go` with handler.
- Validate params/body; enforce timeout bounds and idempotency.
- Poll workflow repo with backoff (200ms → 5s, ±10% jitter) until terminal or timeout.
- Return `{ workflow: State, output, exec_id }` (200) or `{ error, data?: { exec_id, state } }` (408).
- Use `logger.FromContext(ctx)` and `config.FromContext(ctx)`; no globals.
</requirements>

## Subtasks

- [ ] 5.1 Handler + request validation
- [ ] 5.2 Repo polling with backoff and early-exit on terminal
- [ ] 5.3 Idempotency and error mapping (400/404/408/409)
- [ ] 5.4 Unit tests for success and timeout paths

## Sequencing

- Blocked by: 2.0
- Unblocks: 6.0, 7.0, 8.0, 9.0
- Parallelizable: No (touches workflow infra)

## Implementation Details

See Tech Spec “Implementation Design → Data Models” and “Performance”. Ensure worker readiness checks and consistent response envelopes.

### Relevant Files

- `engine/workflow/router/execute_sync.go`
- `engine/infra/server/router/idempotency.go`

### Dependent Files

- `engine/infra/server/router/helpers.go`
- `engine/workflow/router/workflows.go`

## Success Criteria

- Endpoint meets PRD R1–R7 including 408 behavior
- Lints/tests pass
