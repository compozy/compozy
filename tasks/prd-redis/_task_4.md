---
status: pending
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>redis, lua, cache, locking</dependencies>
<unblocks>5.0, 6.0, 8.0</unblocks>
</task_context>

# Task 4.0: Implement Task repository on Redis (behavior parity)

## Overview

Implement Redis-backed task state repo including parent/child mappings, multiple query surfaces, ordered views, and lock semantics.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Use Lua for UpsertState covering all affected indexes and sentinel `:idx` hash
- Implement `WithTransaction` via WATCH+TxPipelined and/or Lua
- Implement `GetStateForUpdate` using lock manager with metrics
- Maintain created_at ZSETs per workflow and per parent
</requirements>

## Subtasks

- [ ] 4.1 Implement `taskrepo.go` with Upsert/Get/List methods and indexes
- [ ] 4.2 Implement distributed locking via `engine/infra/cache`
- [ ] 4.3 Property tests for index invariants and ordering; race tests

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 6.0, 8.0
- Parallelizable: Yes

## Implementation Details

See tech spec sections 3.3, 4, 5, 6, 7, 10, 11.

### Relevant Files

- `engine/infra/redis/taskrepo.go`

### Dependent Files

- `engine/infra/repo/provider.go`

## Success Criteria

- Behavior parity with Postgres path; lock metrics present; tests and lints green
