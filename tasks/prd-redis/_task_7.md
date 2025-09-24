---
status: pending
parallelizable: true
blocked_by: ["5.0"]
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>low</complexity>
<dependencies>telemetry, health</dependencies>
<unblocks>8.0, 10.0</unblocks>
</task_context>

# Task 7.0: Add metrics/monitoring labels and health checks

## Overview

Add `store_driver=redis` labels, repository operation counters, and ensure Redis health checks at startup.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Startup health check via PING
- Add repo op metrics comparable to Postgres baseline
- Ensure logs avoid sensitive values
</requirements>

## Subtasks

- [ ] 7.1 Label telemetry with `store_driver=redis`
- [ ] 7.2 Add repo op metrics and interceptors if needed
- [ ] 7.3 Health check wiring tests

## Sequencing

- Blocked by: 5.0
- Unblocks: 8.0, 10.0
- Parallelizable: Yes

## Implementation Details

PRD Goals (observability) and tech spec 7, 9.

### Relevant Files

- `engine/infra/monitoring/*`
- `engine/infra/server/dependencies.go`

### Dependent Files

- `engine/infra/redis/*`

## Success Criteria

- Metrics visible; health checks pass; tests and lints green
