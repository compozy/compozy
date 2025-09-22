## markdown

## status: completed

<task_context>
<domain>engine/model/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 17.0: Models — Typed Top‑Level Endpoints

## Overview

Convert models list/get/put/delete to typed DTO payloads.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `ModelDTO`, `ModelListItem`, `ModelsListResponse` in `engine/model/router/dto.go`.
- Refactor `engine/model/router/models.go` to return typed responses and ETag header for single.
- Update Swagger annotations.
</requirements>

## Subtasks

- [x] 17.1 Add DTOs/mappers.
- [x] 17.2 Refactor handlers; preserve ETag.
- [x] 17.3 Update Swagger and regenerate.

## Implementation Details

Follow the envelope + header semantics; no anonymous Swagger object types.

### Relevant Files

- `engine/model/router/models.go`
- `engine/model/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- Typed responses in `/api/v0/models*`; docs updated.
