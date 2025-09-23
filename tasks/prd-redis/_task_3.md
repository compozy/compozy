---
status: pending
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/workflow</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>redis, lua, cache</dependencies>
<unblocks>5.0, 8.0</unblocks>
</task_context>

# Task 3.0: Implement Workflow repository on Redis (behavior parity)

## Overview

Implement Redis-backed workflow state repo with required indexes and created-at ordering views.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Preserve interfaces and external behaviors
- Use ZSET for created_at ordering; maintain per-status sets
- Apply TTLs only to terminal workflow states (configurable)
</requirements>

## Subtasks

- [ ] 3.1 Implement `workflowrepo.go` with Upsert/UpdateStatus/Get/List
- [ ] 3.2 Maintain created_at ZSET and status indexes
- [ ] 3.3 Unit/property tests for index consistency and ordering

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 8.0
- Parallelizable: Yes

## Implementation Details

See tech spec sections 3.2, 4, 5, 6, 7, 10.

### Relevant Files

- `engine/infra/redis/workflowrepo.go`

### Dependent Files

- `engine/infra/repo/provider.go`

## Success Criteria

- Behavior parity; correct ordering; tests and lints green
