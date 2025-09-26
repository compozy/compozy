---
status: pending
parallelizable: true
blocked_by: ["3.0", "4.0", "5.0", "8.0"]
---

<task_context>
<domain>engine/\*</domain>
<type>testing</type>
<scope>integration</scope>
<complexity>high</complexity>
<dependencies>worker,temporal,redis</dependencies>
<unblocks>10.0</unblocks>
</task_context>

# Task 9.0: Integration tests: sync workflow + async flows

## Overview

Add integration tests that execute the happy-path and timeout scenarios end-to-end for the new endpoints, including async status polling.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Sync workflow: run lightweight workflow → 200; set low timeout → 408 while exec continues running.
- Agent/Task async: `POST /.../async` → 202 + `Location`, then `GET /executions/.../{exec_id}` until terminal.
- Idempotency: duplicate `X-Idempotency-Key` returns prior outcome without duplicate execution.
</requirements>

## Subtasks

- [ ] 9.1 Sync workflow success/timeout scenarios
- [ ] 9.2 Agent/Task async status flow
- [ ] 9.3 Idempotency duplicate behavior

## Sequencing

- Blocked by: 3.0, 4.0, 5.0, 8.0
- Unblocks: 10.0
- Parallelizable: Yes (with infra up)

## Implementation Details

Reuse existing test harness for server and worker readiness. Keep tests hermetic.

### Relevant Files

- `engine/*/router/*.go`
- `engine/worker/*`

### Dependent Files

- `test/*` (helpers)

## Success Criteria

- End-to-end flows validated; flake-free
- Lints/tests pass
