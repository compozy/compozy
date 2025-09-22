## markdown

## status: completed

<task_context>
<domain>engine/memoryconfig/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 19.0: Memories — Typed Top‑Level Endpoints

## Overview

Convert memories config list/get/put/delete to typed DTO payloads.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `MemoryDTO`, `MemoryListItem`, `MemoriesListResponse` in `engine/memoryconfig/router/dto.go`.
- Refactor `engine/memoryconfig/router/memories.go` to return typed responses and ETag header for single.
- Update Swagger annotations.
</requirements>

## Subtasks

- [x] 19.1 Add DTOs/mappers.
- [x] 19.2 Refactor handlers; preserve ETag.
- [x] 19.3 Update Swagger and regenerate.

## Implementation Details

Typed list response + ETag header on single responses.

### Relevant Files

- `engine/memoryconfig/router/memories.go`
- `engine/memoryconfig/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- Typed responses in `/api/v0/memories*`; docs updated.
