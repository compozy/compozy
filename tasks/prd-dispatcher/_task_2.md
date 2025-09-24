---
status: pending
parallelizable: false
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/worker</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal,redis</dependencies>
<unblocks>"3.0","6.0"</unblocks>
</task_context>

# Task 2.0: Supervisor for Stale Dispatcher Reconciliation

## Overview

Create a supervisor that runs on worker startup and every minute to reconcile stale dispatchers by inspecting Redis heartbeat keys, classifying stale vs. healthy via TTL metadata, terminating associated Temporal workflows, and deleting keys.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/architecture.mdc, @.cursor/rules/go-coding-standards.mdc</import>

<requirements>
- Run supervisor at startup and at a fixed interval (default 1 minute).
- Classify heartbeat entries with TTL-based thresholding (configurable).
- Terminate associated Temporal workflows for stale entries before deleting heartbeat keys.
- Emit metrics: `dispatcher.active`, `dispatcher.stale`, `dispatcher.terminations`, `dispatcher.cleanup_errors`.
</requirements>

## Subtasks

- [ ] Implement supervisor loop with context-aware ticker and shutdown.
- [ ] Implement TTL classification utilities using existing Redis helpers.
- [ ] Implement termination of associated Temporal workflows, then key deletion.
- [ ] Emit metrics for active, stale, terminations, cleanup errors.
- [ ] Unit tests: classification, termination decision logic, error handling.
- [ ] Integration test: crash simulation -> cleanup within ≤ 2 minutes (95th) in staging env.

## Sequencing

- Blocked by: 1.0
- Unblocks: 3.0, 6.0
- Parallelizable: No (depends on deterministic ownership)

## Implementation Details

Ensure context inheritance, `logger.FromContext(ctx)`, `config.FromContext(ctx)`. Add rate limiting and staggering to avoid overload.

### Relevant Files

- `engine/worker/supervisor.go` (create)
- `engine/infra/redis/*`
- `engine/worker/mod.go`

### Dependent Files

- `engine/infra/monitoring/*`

## Success Criteria

- Stale cleanup SLA ≤ 2 minutes (95th percentile).
- Metrics reflect accurate counts of active/stale/terminations and cleanup errors.
