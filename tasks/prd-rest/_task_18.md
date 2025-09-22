## markdown

## status: completed

<task_context>
<domain>engine/schema/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 18.0: Schemas — Typed Top‑Level Endpoints

## Overview

Convert schemas list/get/put/delete to typed DTO payloads. Use `json.RawMessage` for schema body and enforce reasonable size limits.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `SchemaDTO` (with `Body json.RawMessage`), `SchemaListItem`, `SchemasListResponse` in `engine/schema/router/dto.go`.
- Refactor `engine/schema/router/schemas.go` to return typed responses and set ETag header; remove any body `_etag` logic for single.
- Update Swagger annotations; provide example bodies where feasible.
</requirements>

## Subtasks

- [x] 18.1 Add DTOs (SchemaDTO with RawMessage body) and mappers.
- [x] 18.2 Refactor handlers; enforce size guard; preserve ETag.
- [x] 18.3 Update Swagger and regenerate.

## Implementation Details

Avoid deep interface{} decoding; enforce size limit before marshal.

### Relevant Files

- `engine/schema/router/schemas.go`
- `engine/schema/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- Typed responses in `/api/v0/schemas*`; docs updated; size constraints documented.
