# RFC/Plan: Strongly-Typed Resource Routes (Top-Level CRUD)

Status: Draft
Date: 2025-09-21
Owner: Platform/API
Scope: Server API, Routers, Swagger, Tests, CLI touch-points

## Summary

We will migrate the top-level resource endpoints to return strongly-typed DTOs instead of `map[string]any`, while preserving strong ETag concurrency, headers, pagination, and conflict semantics. We will keep the existing envelope (`router.Response`) and place typed data inside its `data` field. For single-item responses, the ETag remains in the response headers; for list endpoints, we will carry per-item ETags via a typed list item wrapper to avoid polluting domain DTOs.

<critical>
Greenfield is the selected approach. BEFORE DO ANYTHING, we will remove the legacy `fields=` feature from the entire project (handlers, helpers, tests, and Swagger).
</critical>

This plan covers: workflows, agents, tools, tasks, mcps, schemas, models, and memories. It uses an incremental per‑resource rollout within the greenfield handler set.

## Goals

- Strongly-typed API responses for all top-level resource routes
- Preserve optimistic concurrency with strong ETags (`If-Match` → 412 on mismatch)
- Maintain cursor pagination and `Link` headers on lists
- Keep response envelope; typed payloads live in `data`
- Per resource: keep DTOs colocated under the router package for clean architecture boundaries (or `engine/{resource}/dto`) and keep mappers pure (no `gin` imports)
- Update Swagger annotations to reflect typed shapes

## Non-Goals

- Introducing content-negotiation (e.g., `profile=typed`) — not desired
- Changing authentication/authorization or rate-limiting behavior
- Refactoring internal store interfaces (`engine/resources`) beyond necessary mapping helpers

## Current State (quick reference)

- Top-level routers for workflows/tools/agents/models/schemas use `map[string]any` in responses and annotations.
- ETag semantics implemented: `If-Match` parsing and strong comparisons; single-item responses set `ETag` header; list responses add `_etag` into each item body.
- `fields=` sparse fieldset logic relies on map-based projection.
- Use-cases (`engine/{resource}/uc`) already decode stored values into concrete `*Config` types before converting back to maps via `AsMap()`.
- Swagger annotations already demonstrate typed envelopes in some nested routes (e.g., workflow/tool nested endpoints).

## High-Level Design

- Keep `router.Response` as the common envelope; do not alter its fields.
- Define DTOs per resource in the resource router package: `engine/{resource}/router/dto.go` (or `engine/{resource}/dto`).
- Lists use named wrappers:
  - `<Resource>ListItem` — transport metadata + resource DTO, e.g., `ETag string `json:"\_etag"`, `<Resource> <Resource>DTO`.
  - `PageInfoDTO` — shared pagination info (limit, total, next_cursor, prev_cursor).
  - `<Resource>ListResponse` — `{ <resources>: []<Resource>ListItem, page: PageInfoDTO }`.
    Single GET/PUT set `ETag` only via header (no `_etag` in body).
- Replace map-based projection with typed responses. The legacy `fields=` query is removed project‑wide (Step 0).
- Continue to accept and return the same request/response headers (ETag, Link, RateLimit-\*). Keep `Location` on 201 Created.

## DTO Layout (per resource)

Create colocated DTOs per router package to avoid leaking transport concerns into domain types:

- `engine/workflow/router/dto.go`
  - `WorkflowDTO` — stable top-level fields (id, version, description, author, config opts, triggers, schedule, counts, id arrays, expandable collections).
  - `WorkflowListItem` — embed core fields at root and include `etag` (Tool DTO pattern).
  - `WorkflowsListResponse` — `{ Workflows []WorkflowListItem `json:"workflows"`, Page PageInfoDTO `json:"page"` }`.
- `engine/tool/router/dto.go`
  - `ToolDTO` — maps from `tool.Config` (id, description, timeout, input/output schema, with, config, env, cwd where appropriate).
  - `ToolListItem` — `{ ETag string `json:"\_etag"`, Tool ToolDTO `json:"tool"` }`.
  - `ToolsListResponse` — `{ Tools []ToolListItem `json:"tools"`, Page PageInfoDTO `json:"page"` }`.
- `engine/agent/router/dto.go`
  - `AgentDTO` — from `agent.Config` (id, llm config, memory refs, tools, mcps, params, etc.).
  - `AgentListItem`, `AgentsListResponse`.
- `engine/task/router/dto.go`
  - `TaskDTO` — from `task.Config` (base config + type-specific projections as needed for top-level CRUD representation).
  - `TaskListItem`, `TasksListResponse`.
- `engine/mcp/router/dto.go`
  - `MCPDTO`, `MCPListItem`, `MCPsListResponse`.
- `engine/model/router/dto.go`
  - `ModelDTO`, `ModelListItem`, `ModelsListResponse`.
- `engine/schema/router/dto.go`
  - `SchemaDTO` — typed frame (id, description, metadata) plus dynamic schema body as `json.RawMessage`.
  - `SchemaListItem`, `SchemasListResponse`.
- `engine/memoryconfig/router/dto.go` (for top-level memory configs)
  - `MemoryDTO`, `MemoryListItem`, `MemoriesListResponse`.

Notes:

- Introduce a shared `PageInfoDTO` (limit, total, next_cursor, prev_cursor) in a common package.
- DTOs are transport-facing; keep domain `Config` types untouched. Mappers must not import `gin`.
- For workflow `expand` collections, typed union fields are used with custom JSON marshaling to return either `[]string` (IDs) or typed DTO slices, while keeping the `tasks|agents|tools` property names stable.

## Router Changes (per resource)

For each `engine/{resource}/router/*.go` top-level route file:

- Replace map construction with DTO mapping functions. Use existing UC decode helpers (e.g., `decodeStoredTool`) to get `*Config`, then map to DTO.
- Single-item GET/PUT:
  - Response: `router.RespondOK/Created(c, message, dto)`
  - Header: `c.Header("ETag", string(out.ETag))`
  - Remove body `_etag` for single-item returns (header only).
- List endpoints:
  - Preserve limit/cursor/q filters and `Link` headers.
  - Map each window item to `<Resource>ListItem`.
  - Build a concrete `<Resource>ListResponse` (avoid `gin.H`).
  - Response: `router.RespondOK(c, "<resources> retrieved", <Resource>ListResponse{ ... })`.
- Remove `fields=` from handlers/Swagger entirely (feature deleted in Step 0). Continue supporting `q` and `expand` where applicable (e.g., workflows).

## Swagger Annotations

Update annotations to reflect typed payloads and preserve envelope:

- Single GET:
  - `@Success 200 {object} router.Response{data=workrouter.WorkflowDTO} "Workflow retrieved"`
- List GET (named types only; no `object{...}`):
  - `@Success 200 {object} router.Response{data=workrouter.WorkflowsListResponse} "Workflows retrieved"`
- PUT (upsert):
  - `@Success 200 {object} router.Response{data=workrouter.WorkflowDTO} "Workflow updated"`
  - `@Success 201 {object} router.Response{data=workrouter.WorkflowDTO} "Workflow created"`
- DELETE:
  - unchanged (204)

Apply equivalent patterns to tools, agents, tasks, mcps, models, schemas, and memories.

Regenerate Swagger as part of CI step to ensure typed schemas appear under components.

## Migration Strategy (Greenfield)

0. MANDATORY PRE-WORK — Remove `fields=` feature across the project BEFORE anything else
   - Delete `ParseFieldsQuery`, `FilterMapFields`, and any call sites.
   - Remove `fields` parameter from Swagger annotations and docs.
   - Update tests relying on `fields=` to use typed DTOs or expanded views instead.
   - Grep locations (routers, helpers, tests) and purge the feature completely.

1. Baseline health: `make lint && make test`.

2. Pilot resource: Tools (typed-only handlers)
   - Add `engine/tool/router/dto.go` with `ToolDTO`, `ToolListItem`, `ToolsListResponse`, and `PageInfoDTO` (shared).
   - Refactor `engine/tool/router/tools_top.go` to return typed DTOs and typed list responses; no `gin.H`.
   - Update Swagger annotations to named typed forms; regen docs.
   - Update integration tests under `test/integration/resources/tools_test.go` to decode into typed structs.

3. Workflows (typed-only)
   - Extend `engine/workflow/router/dto.go` with `WorkflowListItem`, `WorkflowsListResponse`.
   - Update `workflows.go` for typed returns; keep `expand`.
   - Update `test/integration/resources/workflows_test.go` accordingly (remove any `fields` remnants; keep pagination/link/etag checks).

4. Agents, Tasks, Models, Schemas, Memories
   - Add DTOs and refactor routers similarly; adjust tests and docs.

5. Clean-up & Consistency
   - Ensure all Swagger annotations reference named DTOs (no anonymous `object{...}` forms) and add examples where helpful.

6. Final baseline: `make lint && make test`.

## CI Hygiene

- Add Swagger generation/format checks to CI:
  - Run swag format/generation (or project make target) and `git diff --exit-code` to fail on drift.
- Keep consistent header annotations (ETag, Link, RateLimit-\*).

## Tests & Backward Compatibility

- Integration tests in `test/integration/resources/*` currently rely on map-based responses and sometimes `fields=`. We will:
  - Remove `fields` assertions entirely (feature removed in Step 0).
  - Update helpers to decode typed DTOs (single and list forms).
  - Keep pagination/Link headers and ETag checks unchanged.
  - Keep conflict and error semantics unchanged (Problem Details, 409/412/404).
- The project is in a non-BC phase (development). Document the response shape changes and the removal of `fields=` in release notes for consumers.

## Risks & Mitigations

- Removal of `fields=` flexibility → Explicitly removed by design; if needed later, consider a typed projection or GraphQL in a future RFC.
- Schema resource dynamism → Restrict the dynamic part to schema body within a typed frame.
- Swagger drift → Enforce typed annotations and add CI check for swagger generation.

## Work Breakdown

- Tools (pilot)
  - DTOs + mapper + router refactor + Swagger + tests
- Workflows
  - DTO extension + router refactor (+ retain `expand`) + Swagger + tests
- Agents, Tasks, Models, Schemas, Memories
  - Repeat pattern
- Cleanup
  - Remove map projections from typed handlers, consolidate helpers

## Acceptance Criteria

- All top-level resource endpoints return typed DTOs inside the standard `router.Response` envelope
- Single GET/PUT set `ETag` header; list endpoints include per-item `_etag` within typed list item wrappers
- `fields=` feature removed from codebase, tests, and docs
- Swagger reflects named typed schemas across all endpoints; docs build passes
- All tests pass: `make lint && make test`

## Implementation Notes (code refs)

- Map → DTO switch points:
  - Workflows: `engine/workflow/router/workflows.go`
  - Tools: `engine/tool/router/tools_top.go`
  - Agents: `engine/agent/router/agents_top.go`
  - Models: `engine/model/router/models.go`
  - Schemas: `engine/schema/router/schemas.go`
- UC mapping helpers exist (e.g., `decodeStoredTool`) to get `*Config`; then map to DTO.
- Envelope & errors: keep `engine/infra/server/router/response.go` and `problem.go` unchanged.

### Example (Tools)

```go
// Shared page info
type PageInfoDTO struct {
    Limit      int    `json:"limit"`
    Total      int    `json:"total"`
    NextCursor string `json:"next_cursor,omitempty"`
    PrevCursor string `json:"prev_cursor,omitempty"`
}

// Tool DTOs
type ToolDTO struct { /* fields mapped from tool.Config */ }

type ToolListItem struct {
    ETag string  `json:"_etag"`
    Tool ToolDTO `json:"tool"`
}

type ToolsListResponse struct {
    Tools []ToolListItem `json:"tools"`
    Page  PageInfoDTO    `json:"page"`
}

// Handler: build typed response instead of gin.H
router.RespondOK(c, "tools retrieved", ToolsListResponse{Tools: items, Page: page})
```

---

Prepared by: API Platform
