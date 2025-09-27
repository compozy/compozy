---
status: completed
parallelizable: true
blocked_by: ["2.0", "3.0", "4.0", "5.0"]
---

<task_context>
<domain>engine/\*/router</domain>
<type>testing</type>
<scope>unit</scope>
<complexity>medium</complexity>
<dependencies>testing,testify</dependencies>
<unblocks>9.0</unblocks>
</task_context>

# Task 8.0: Unit tests: validation, idempotency, responses

## Overview

Add unit tests for router validation, idempotency helper behavior, and response mapping for all new endpoints.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Follow `.cursor/rules/test-standards.mdc` (`t.Run("Should...")`, testify).
- Agent/Task: validation (missing action/prompt), sync timeout bounds, idempotency duplicate → 409.
- Workflow sync: success (200) and server-side timeout (408) mapping.
- No workarounds or globals; use proper mocks/adapters.
</requirements>

## Subtasks

- [x] 8.1 Idempotency helper tests (header vs hash, TTL)
- [x] 8.2 Agent/Task router tests (validation, 200/202/400/404/409)
- [x] 8.3 Workflow sync router tests (200/408)

## Sequencing

- Blocked by: 2.0, 3.0, 4.0, 5.0
- Unblocks: 9.0
- Parallelizable: Yes

## Implementation Details

Mirror patterns from `engine/webhook/*_test.go` and existing router tests.

### Relevant Files

- `engine/infra/server/router/idempotency.go`
- `engine/agent/router/exec.go`
- `engine/task/router/exec.go`
- `engine/workflow/router/execute_sync.go`

### Dependent Files

- `engine/webhook/service_test.go`
- `engine/tool/router/router_test.go`

## Success Criteria

- High-confidence unit coverage for new paths
- Lints/tests pass
