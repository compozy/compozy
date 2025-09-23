---
status: pending
parallelizable: false
blocked_by: []
---

<task_context>
<domain>engine/infra/redis</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>redis, configuration</dependencies>
<unblocks>2.0, 3.0, 4.0, 10.0</unblocks>
</task_context>

# Task 1.0: Create Redis infra scaffolding (keys, lua, helpers)

## Overview

Introduce `engine/infra/redis` package with key builders, namespacing, TTL helpers, and Lua script registration for atomic multi-key upserts and index maintenance.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use `logger.FromContext(ctx)` and `config.FromContext(ctx)`
- No global singletons; pass Redis client via provider wiring
- Key namespace: `compozy:v1:<env>:<domain>:...` with optional tenant hash tag
- Provide helpers for ZSET timestamps (unix seconds), stable JSON, and script loading
</requirements>

## Subtasks

- [ ] 1.1 Add `keys.go` with builders and TTL constants
- [ ] 1.2 Add `lua.go` with script registration and invocation helpers
- [ ] 1.3 Add JSON marshal helpers (StableJSONBytes) and timestamp utilities
- [ ] 1.4 Unit tests using `miniredis` for script registration stubs

## Sequencing

- Blocked by: —
- Unblocks: 2.0, 3.0, 4.0, 10.0
- Parallelizable: No (foundation for others)

## Implementation Details

See tech spec sections 3, 4, 6 for key schema, Lua atomicity, and serialization rules.

### Relevant Files

- `engine/infra/redis/keys.go`
- `engine/infra/redis/lua.go`

### Dependent Files

- `engine/infra/repo/provider.go`
- `engine/infra/server/dependencies.go`

## Success Criteria

- Helpers compile and are covered by unit tests
- Keys follow namespace rules; Lua registry is testable; linters/tests pass
