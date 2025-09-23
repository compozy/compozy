---
status: pending
parallelizable: true
blocked_by: ["4.0", "5.0"]
---

<task_context>
<domain>engine/workflow,engine/task</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>redis, ttl</dependencies>
<unblocks>8.0</unblocks>
</task_context>

# Task 6.0: Implement TTL/retention for terminal states

## Overview

Apply configurable TTLs to terminal task and workflow states per PRD guidance. Auth objects remain without TTL.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Tasks: TTL 7–30d; Workflows: TTL 30–90d (configurable)
- Ensure TTL set on terminal transitions; do not set for non-terminal
- Auth: no TTL
</requirements>

## Subtasks

- [ ] 6.1 Add config wiring for retention windows
- [ ] 6.2 Apply TTL on terminal transitions in repos
- [ ] 6.3 Tests for TTL application and no-ttl cases

## Sequencing

- Blocked by: 4.0, 5.0
- Unblocks: 8.0
- Parallelizable: Yes

## Implementation Details

See PRD R9 and tech spec sections 3.2/3.3 and 9 (retention).

### Relevant Files

- `engine/infra/redis/workflowrepo.go`
- `engine/infra/redis/taskrepo.go`

### Dependent Files

- `pkg/config/*`

## Success Criteria

- TTLs applied correctly per state; tests and lints green
