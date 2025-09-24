---
status: pending
parallelizable: false
blocked_by: ["2.0", "3.0", "4.0"]
---

<task_context>
<domain>engine/infra</domain>
<type>integration</type>
<scope>middleware</scope>
<complexity>medium</complexity>
<dependencies>server, repo provider, cache</dependencies>
<unblocks>6.0, 7.0, 8.0, 10.0</unblocks>
</task_context>

# Task 5.0: Replace provider and server wiring with Redis-only

## Overview

Switch provider and server dependencies to construct Redis client, perform health check, and wire Redis-backed repos; remove Postgres wiring.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `config.FromContext(ctx)` for Redis config
- Health check via PING; label telemetry `store_driver=redis`
- Remove Postgres wiring paths (no toggle)
</requirements>

## Subtasks

- [ ] 5.1 Update `engine/infra/repo/provider.go` to Redis-only
- [ ] 5.2 Update `engine/infra/server/dependencies.go` to build Redis client
- [ ] 5.3 Ensure make dev/test paths use Redis only

## Sequencing

- Blocked by: 2.0, 3.0, 4.0
- Unblocks: 6.0, 7.0, 8.0, 10.0
- Parallelizable: No (central wiring)

## Implementation Details

See tech spec section 2 and 8.

### Relevant Files

- `engine/infra/repo/provider.go`
- `engine/infra/server/dependencies.go`

### Dependent Files

- `engine/infra/redis/*`

## Success Criteria

- App boots with Redis; telemetry label present; tests and lints green
