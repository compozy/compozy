---
status: pending
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/worker</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal</dependencies>
<unblocks>"2.0","4.0"</unblocks>
</task_context>

# Task 1.0: Deterministic Dispatcher Ownership and Takeover

## Overview

Implement deterministic dispatcher ownership so that only one healthy dispatcher is active per project/task queue. Use stable workflow IDs and a termination-first startup to evict stale executions and guarantee takeover.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/architecture.mdc, @.cursor/rules/go-coding-standards.mdc</import>

<requirements>
- Derive dispatcher workflow IDs from `dispatcher-<project>-<task-queue>`.
- Start/take over dispatcher with a termination-first policy that ensures stale executions are ended before proceeding.
- Include `serverID` in workflow input metadata (not in the workflow ID).
- Record takeover telemetry: counter + latency histogram.
- Handle legacy `WorkflowExecutionAlreadyStarted` by issuing `TerminateWorkflow` then retrying startup.
</requirements>

## Subtasks

- [ ] Implement a stable ID builder for `dispatcher-<project>-<task-queue>`.
- [ ] Update dispatcher startup to use termination-first policy (`SignalWithStart` + TerminateIfRunning semantics; fallback: explicit terminate-then-retry).
- [ ] Add telemetry for takeover count and latency.
- [ ] Integrate new startup path at worker boot.
- [ ] Unit tests: ID builder, takeover branch (success, AlreadyStarted→Terminate→retry).
- [ ] Integration test: restart replaces stale dispatcher within one attempt; history shows termination before new execution.

## Sequencing

- Blocked by: None
- Unblocks: 2.0, 4.0
- Parallelizable: No (foundational behavior)

## Implementation Details

High-level behavior only; see Tech Spec for API details. Ensure context propagation, `logger.FromContext(ctx)` and `config.FromContext(ctx)` usage.

### Relevant Files

- `engine/worker/mod.go`
- `engine/worker/dispatcher.go` (create if missing)
- `engine/infra/server/lifecycle.go` (reference)

### Dependent Files

- `engine/infra/monitoring/*`
- `engine/infra/redis/*`

## Success Criteria

- p95 takeover latency < 30s during worker restarts.
- Temporal history shows termination preceding new execution on takeover.
- Duplicate dispatcher count = 0 during rolling restarts.
