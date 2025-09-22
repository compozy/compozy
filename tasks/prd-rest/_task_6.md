---
status: completed
parallelizable: false
blocked_by: ["2.0", "3.0", "5.0"]
---

<task_context>
<domain>engine/resources/router</domain>
<type>cleanup</type>
<scope>deprecation|removal</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"7.0","8.0","10.0"</unblocks>
</task_context>

# Task 6.0: Remove Generic `/resources` Router

## Overview

Unregister and remove the legacy generic `/resources` HTTP router and associated tests. Ensure Swagger surface no longer exposes these endpoints.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Remove import and call to `resourcesrouter.Register(apiBase)` from `reg_components.go`.
- Delete `engine/resources/router/*` and related tests.
- Update Swagger to remove tags/paths for `/resources`.
</requirements>

## Subtasks

- [x] 6.1 Update `engine/infra/server/reg_components.go` to drop resources router.
- [x] 6.2 Delete `engine/resources/router/*` package and tests.
- [x] 6.3 Validate server boots without `/resources`; run tests.

## Sequencing

- Blocked by: 2.0, 3.0, 5.0
- Unblocks: 7.0, 8.0, 10.0
- Parallelizable: No (central registration change)

## Implementation Details

Search for `resourcesrouter` references and remove safely.

### Relevant Files

- `engine/infra/server/reg_components.go`
- `engine/resources/router/*`

### Dependent Files

- `docs/swagger.yaml`

## Success Criteria

- No references to `resourcesrouter` remain; server compiles and runs; tests pass.
