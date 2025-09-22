## markdown

## status: completed

<task_context>
<domain>engine/workflow/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 14.0: Workflows — Typed Top‑Level Endpoints

## Overview

Convert workflows list/get/put/delete to typed DTO payloads. Retain `expand=` behavior.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Extend `engine/workflow/router/dto.go` with `WorkflowListItem`, `WorkflowsListResponse`.
- Update `engine/workflow/router/workflows.go` to:
  - Remove any `fields` logic (handled by Task 11.0).
  - Build typed responses; set ETag header for single.
- Update Swagger annotations to named types and headers.
- Update integration tests in `test/integration/resources/workflows_test.go` (remove `fields` assertions; keep pagination/Link/ETag checks).
</requirements>

## Subtasks

- [x] 14.1 Add DTOs/mappers and list response.
- [x] 14.2 Refactor handlers to return typed payloads.
- [x] 14.3 Swagger updates and docs regeneration.
- [x] 14.4 Update tests.

## Implementation Details

Preserve cursor pagination + Link headers; ETag on single via headers.

### Relevant Files

- `engine/workflow/router/workflows.go`
- `engine/workflow/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- `/api/v0/workflows` and `/api/v0/workflows/{id}` return typed responses; ETag/Link intact.
- Swagger reflects named DTOs; tests pass.
