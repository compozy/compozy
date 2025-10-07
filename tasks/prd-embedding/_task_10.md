---
status: pending
parallelizable: false
blocked_by: ["5.0","6.0","8.0"]
---

<task_context>
<domain>engine/infra/server/router/knowledge</domain>
<type>implementation|testing</type>
<scope>api</scope>
<complexity>medium</complexity>
<dependencies>http_server|database</dependencies>
<unblocks>"11.0","13.0","15.0"</unblocks>
</task_context>

# Task 10.0: HTTP APIs

## Overview
Expose CRUD for knowledge bases and actions for ingestion and query with ETag, pagination, and problem+json error envelopes.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Routes: `/knowledge-bases` CRUD; `POST /knowledge-bases/{id}/ingest`; `POST /knowledge-bases/{id}/query`.
- Enforce conditional requests (ETag) and standard pagination.
- Generate Swagger paths/tags; golden tests must validate exported API.
- Unit tests for ETag, pagination, validation errors.
- Run `make fmt && make lint && make test` before completion.
</requirements>

## Subtasks
- [ ] 10.1 Implement router and handlers
- [ ] 10.2 Unit tests `engine/infra/server/router/knowledge_router_test.go`
  - ETag preconditions and conditional GET (304)
  - Pagination forward/backward symmetry
  - Body validation and problem+json
- [ ] 10.3 Swagger generation/parity smoke test

## Sequencing
- Blocked by: 5.0, 6.0, 8.0
- Unblocks: 11.0, 13.0, 15.0
- Parallelizable: No

## Implementation Details
Follow repository API standards for errors and pagination; keep responses stable for golden tests.

### Relevant Files
- `engine/infra/server/router/knowledge/*`
- `docs/swagger/*` (if applicable)

### Dependent Files
- `cli/cmd/knowledge/*`

## Success Criteria
- Endpoints compile; unit tests pass; Swagger reflects routes.
