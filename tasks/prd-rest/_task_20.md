## markdown

## status: pending

<task_context>
<domain>engine/project/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 20.0: Project — Typed Top‑Level Endpoint

## Overview

Convert `/project` get/put to typed DTO response and remove any map filtering.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Define `ProjectDTO` (minimum fields needed by API contract) in `engine/project/router/dto.go`.
- Refactor `engine/project/router/project.go` to return typed DTO; remove any `FilterMapFields` usage.
- Update Swagger annotations (headers, examples).
</requirements>

## Subtasks

- [ ] 20.1 Add `ProjectDTO` and mapper.
- [ ] 20.2 Refactor handlers to return typed payloads.
- [ ] 20.3 Update Swagger annotations and run generation.

## Implementation Details

Follow the typed envelope pattern and preserve ETag/Location semantics where applicable.

### Relevant Files

- `engine/project/router/project.go`
- `engine/project/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- Typed responses for `/api/v0/project`; docs updated.
