---
status: completed
parallelizable: true
blocked_by: ["3.0", "4.0", "5.0"]
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>observability</scope>
<complexity>medium</complexity>
<dependencies>monitoring,http_server</dependencies>
<unblocks>7.0, 8.0, 9.0</unblocks>
</task_context>

# Task 6.0: Wire metrics for execution endpoints

## Overview

Add counters and timers for the new execution endpoints (latency, timeouts, error rates) using the existing monitoring facilities.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add summary/histogram timers: `http_exec_sync_latency_seconds{kind, outcome}`.
- Add counters: `http_exec_timeouts_total{kind}`, `http_exec_errors_total{kind, code}`.
- Emit metrics in handlers; ensure meter wiring and label consistency.
- Tests for metric registration and basic emission (unit-level).
</requirements>

## Subtasks

- [x] 6.1 Define metrics in monitoring package
- [x] 6.2 Emit in agent/task/workflow handlers
- [x] 6.3 Unit tests for registration and increments

## Sequencing

- Blocked by: 3.0, 4.0, 5.0
- Unblocks: 7.0, 8.0, 9.0
- Parallelizable: Yes

## Implementation Details

Follow patterns from `engine/webhook/metrics.go`. Keep naming consistent and avoid cardinality explosions.

### Relevant Files

- `engine/infra/monitoring/*`
- `engine/agent/router/exec.go`
- `engine/task/router/exec.go`
- `engine/workflow/router/execute_sync.go`

### Dependent Files

- `engine/webhook/metrics.go`

## Success Criteria

- Metrics visible and incremented in tests
- Lints/tests pass
