---
status: pending
parallelizable: false
blocked_by: ["1.0", "2.0"]
---

<task_context>
<domain>engine/worker</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>temporal,redis</dependencies>
<unblocks>"6.0"</unblocks>
</task_context>

# Task 3.0: Verification & Operational Hardening (Worker Registration & Eviction)

## Overview

Extend dispatcher to register incoming workers (server ID + task queue), maintain a single dispatcher heartbeat, and periodically evict workers whose heartbeat timestamps exceed stale thresholds.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/architecture.mdc, @.cursor/rules/go-coding-standards.mdc</import>

<requirements>
- Register workers (server ID + task queue) and maintain active host set.
- Periodically self-check and evict workers exceeding stale thresholds.
- All activities inherit context and use `config.FromContext` and `logger.FromContext`.
- Emit minimal metrics for alerts: takeover count, stale cleanup count, worker eviction count.
</requirements>

## Subtasks

- [ ] Implement worker registration flow and data model.
- [ ] Implement periodic self-check and eviction for stale worker heartbeats.
- [ ] Ensure context and config patterns are followed across calls.
- [ ] Load tests with rolling restarts to validate single dispatcher and evictions.
- [ ] Update ops runbook with registration and eviction semantics.

## Sequencing

- Blocked by: 1.0, 2.0
- Unblocks: 6.0
- Parallelizable: No

## Implementation Details

Leverage existing Redis heartbeat schema; avoid background contexts. Keep metrics minimal and aligned to alerting needs.

### Relevant Files

- `engine/worker/dispatcher.go`
- `engine/infra/redis/*`
- `engine/infra/monitoring/*`

### Dependent Files

- `engine/worker/mod.go`

## Success Criteria

- Single dispatcher maintained during rolling restarts; dead workers evicted.
- Runbook updated and validated by Ops.
