## markdown

## status: completed

<task_context>
<domain>engine/tool/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 13.0: Tools — Typed Top‑Level Endpoints (Pilot)

## Overview

Convert `/tools` and `/tools/{tool_id}` to return typed DTOs with ETag header semantics and typed list responses.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add in `engine/tool/router/dto.go`:
  - `ToolDTO` (map from `tool.Config`), `ToolListItem`, `ToolsListResponse`.
- Update `engine/tool/router/tools_top.go` to:
  - Build `[]ToolListItem` for list; respond with `ToolsListResponse`.
  - Single GET/PUT returns `ToolDTO` and sets `ETag` header; no body `_etag`.
- Update Swagger annotations to use named DTOs (no `object{...}`); include headers.
- Update integration tests in `test/integration/resources/tools_test.go` to validate typed payloads.
</requirements>

## Subtasks

- [x] 13.1 Add DTOs and mappers (`tool.Config` → `ToolDTO`).
- [x] 13.2 Refactor handlers for list/get/put/delete; preserve ETag + conflict behavior.
- [x] 13.3 Update Swagger annotations and regenerate docs.
- [x] 13.4 Update tests.

## Implementation Details

Typed list response + ETag header on single responses.

### Relevant Files

- `engine/tool/router/tools_top.go`
- `engine/tool/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- `/api/v0/tools` and `/api/v0/tools/{id}` return typed responses; headers intact.
- Swagger reflects named DTOs; docs build clean.
- All tests pass.
