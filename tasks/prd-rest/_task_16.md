## markdown

## status: completed

<task_context>
<domain>engine/task/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 16.0: Tasks — Typed Top‑Level Endpoints

## Overview

Convert tasks list/get/put/delete to typed DTO payloads.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `TaskDTO`, `TaskListItem`, `TasksListResponse` in `engine/task/router/dto.go`.
- Refactor `engine/task/router/tasks_top.go` to produce typed responses; ETag header on single.
- Update Swagger annotations.
- Update `test/integration/resources/tasks_test.go` to remove `fields` assertions and validate typed payloads.
</requirements>

## Subtasks

- [x] 16.1 Add DTOs/mappers.
- [x] 16.2 Refactor handlers; preserve ETag.
- [x] 16.3 Update Swagger and regenerate.
- [x] 16.4 Update tests.

## Implementation Details

Typed list response + ETag header on single responses.

### Relevant Files

- `engine/task/router/tasks_top.go`
- `engine/task/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- Typed responses in `/api/v0/tasks*`; docs/tests updated.
