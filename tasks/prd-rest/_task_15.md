## markdown

## status: completed

<task_context>
<domain>engine/agent/router</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 15.0: Agents — Typed Top‑Level Endpoints

## Overview

Convert agents list/get/put/delete to typed DTO payloads.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `AgentDTO`, `AgentListItem`, `AgentsListResponse` in `engine/agent/router/dto.go`.
- Refactor `engine/agent/router/agents_top.go` to produce typed responses; ETag header on single.
- Update Swagger annotations.
- Update `test/integration/resources/agents_test.go` to remove `fields` assertions and validate typed payloads.
</requirements>

## Subtasks

- [x] 15.1 Add DTOs/mappers.
- [x] 15.2 Refactor handlers; preserve ETag.
- [x] 15.3 Update Swagger and regenerate.
- [x] 15.4 Update tests.

## Implementation Details

Typed list response + ETag header on single responses.

### Relevant Files

- `engine/agent/router/agents_top.go`
- `engine/agent/router/dto.go`

### Dependent Files

- `engine/infra/server/router/response.go`

## Success Criteria

- Typed responses in `/api/v0/agents*`; docs/tests updated.
