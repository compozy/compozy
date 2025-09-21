\*\*\*\*# RFC: Replace Generic `/resources` Endpoints With Resource-Specific APIs (Greenfield)

Status: Proposal
Date: 2025-09-21
Owner: Platform/API
Scope: Server API, Routers, Swagger, Tests, CLI touch-points

## Summary

---

We will remove the generic polymorphic HTTP endpoints under `/resources/{type}` and replace them with first‑class, resource‑specific APIs (e.g., `/workflows`, `/tasks`, `/agents`, `/tools`, etc.). The `engine/resources` package remains the source of truth for storage primitives and shared types (stores/meta/locks). Business logic (use‑cases) will live inside each resource package under `engine/{resource}/uc`, and HTTP handlers remain in `engine/{resource}/router`.

This is a greenfield change (alpha stage). No backwards compatibility is required. We will delete the generic `/resources` HTTP router and update Swagger and tests accordingly.

## Goals

- Clear, discoverable API surface modeled as domain resources, not a generic bucket.
- Move business logic (use‑cases) into per‑resource packages (`engine/{resource}/uc`) while keeping storage primitives centralized in `engine/resources`. Resource routers call their local UC, which delegates to the shared store interfaces.
- Unify reads and writes to use the same underlying store to avoid inconsistencies.
- Preserve optimistic concurrency via strong ETags and `If-Match` (HTTP 412 on mismatch), project scoping, and provenance metadata writes.
- Adopt consistent API standards where applicable: keyset cursor pagination + `Link` headers, Problem Details (`application/problem+json`) for errors, and RateLimit response headers.

## Non‑Goals

- Supporting legacy `/resources` endpoints.
- Changing the underlying `ResourceStore` interfaces beyond what’s necessary for correctness.
- Changing authentication model beyond route scoping that becomes simpler with specific routes.

## Current State (as of 2025-09-21)

- Generic router mounts under `/api/v0/resources` and exposes CRUD:
  - POST `/resources/{type}` create
  - GET `/resources/{type}` list keys
  - GET `/resources/{type}/{id}` get
  - PUT `/resources/{type}/{id}` upsert (supports `If-Match`)
  - DELETE `/resources/{type}/{id}` delete
  - Implementation: `engine/resources/router/register.go`, `engine/resources/router/handlers.go`
- Business logic is currently encapsulated in `engine/resources/uc`:
  - Create: `engine/resources/uc/create.go`
  - Upsert: `engine/resources/uc/put.go`
  - Get: `engine/resources/uc/get.go`
  - List: `engine/resources/uc/list.go`
  - Delete: `engine/resources/uc/delete.go`
  - Validations per type (agent, tool, mcp, workflow, project, memory, schema, model): `engine/resources/uc/validators.go`
- Store contracts + implementations: `engine/resources/store.go`, `engine/resources/memory_store.go`, `engine/resources/redis_store.go`, plus meta helpers `engine/resources/meta.go`.
- Specific routers already exist for reads and executions:
  - Workflows: list + get + executions (global and per workflow) in `engine/workflow/router/*`.
  - Agents/Tools/Tasks: read-only nested under `/workflows/{workflow_id}` in `engine/agent/router/*`, `engine/tool/router/*`, `engine/task/router/*`.
- App state and helpers:
  - App state: `engine/infra/server/appstate/state.go`
  - Router helpers (JSON binding, state, store, URL params, standard responses): `engine/infra/server/router/helpers.go`, `engine/infra/server/router/response.go`
  - Routes base helpers: `engine/infra/server/routes/routes.go`

Observations:

- Reads for workflows/agents/tools/tasks currently rely on in-memory compiled configs via `appState.GetWorkflows()`, while writes go through the `ResourceStore` via `/resources` → potential incoherence.
- Generic `/resources` normalizes ETag from `If-Match`, sets `ETag`/`Location` headers, and writes provenance meta. We must preserve these behaviors in the new per-resource handlers.

## Target API (all resources)

This section defines canonical, resource-specific endpoints replacing `/resources/{type}`. All resources share these conventions unless noted:

- Conventions (applies to all):
- Pagination: keyset cursor pagination with `limit` (default 50, max 500) and optional `cursor` query. Cursor tokens encode direction (`after`/`before`) plus the last seen resource ID, base64‑encoded with the `v2:` prefix (e.g., `v2:after:workflow-0001`). Responses include `Link` headers (`rel="next"|"prev"`) and a response body `page.next_cursor`/`page.prev_cursor` when applicable.
  - Filtering: `q=` (ID prefix) remains; add `filter[field]=value` for simple filters. Support sparse fieldsets via `fields=field1,field2`. Embedding via `expand=subresource1,subresource2` where supported.
  - Project: optional `project=` query overrides default project
  - Optimistic concurrency: server issues strong ETags. `PUT` uses strong comparison; on mismatch return `412 Precondition Failed`. On create, respond `201 Created` with `Location`; on update, respond `200 OK` and always set `ETag`.
  - DELETE returns `204 No Content` (idempotent). If a resource is referenced by another resource and cannot be safely removed, return `409 Conflict` with Problem Details describing references.
  - Errors: use Problem Details (`application/problem+json`) across all new endpoints.
  - Rate limiting: include `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` (seconds to reset). Keep existing `X-RateLimit-*` for compatibility.

Workflows

- GET `/executions/workflows` — list all workflow executions (already present)
- GET `/workflows?limit={n}&cursor={cursor}&q={prefix}&fields=...` — list workflows (cursor pagination)
- GET `/workflows/{workflow_id}` — get workflow by ID
- PUT `/workflows/{workflow_id}` — idempotent upsert (supports strong `If-Match`)
- DELETE `/workflows/{workflow_id}` — delete workflow by ID (204)
- GET `/workflows/{workflow_id}/executions` — list executions by workflow (already present)
- POST `/workflows/{workflow_id}/executions` — start/execute workflow (already present)

Tasks (first-class + nested views)

- GET `/tasks?limit={n}&cursor={cursor}&q={prefix}&workflow_id={id}&fields=...`
- GET `/tasks/{task_id}`
- PUT `/tasks/{task_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/tasks/{task_id}` — 204
- Nested reads (DX):
  - GET `/workflows/{workflow_id}/tasks`
  - GET `/workflows/{workflow_id}/tasks/{task_id}`
  - Writes remain canonical at top-level `/tasks/{task_id}` to avoid duplication (optional future alias under workflow)

Agents (first-class + nested views)

- GET `/agents?limit={n}&cursor={cursor}&q={prefix}&workflow_id={id}&fields=...`
- GET `/agents/{agent_id}`
- PUT `/agents/{agent_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/agents/{agent_id}` — 204
- Nested reads remain available:
  - GET `/workflows/{workflow_id}/agents`
  - GET `/workflows/{workflow_id}/agents/{agent_id}`

Tools (first-class + nested views)

- GET `/tools?limit={n}&cursor={cursor}&q={prefix}&workflow_id={id}&fields=...`
- GET `/tools/{tool_id}`
- PUT `/tools/{tool_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/tools/{tool_id}` — 204
- Nested reads remain available:
  - GET `/workflows/{workflow_id}/tools`
  - GET `/workflows/{workflow_id}/tools/{tool_id}`

MCPs

- GET `/mcps?limit={n}&cursor={cursor}&q={prefix}&fields=...`
- GET `/mcps/{mcp_id}`
- PUT `/mcps/{mcp_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/mcps/{mcp_id}` — 204

Schemas

- GET `/schemas?limit={n}&cursor={cursor}&q={prefix}&fields=...`
- GET `/schemas/{schema_id}`
- PUT `/schemas/{schema_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/schemas/{schema_id}` — 204

Models

- GET `/models?limit={n}&cursor={cursor}&q={prefix}&fields=...`
- GET `/models/{model_id}`
- PUT `/models/{model_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/models/{model_id}` — 204

Memory Resources (config) — distinct from runtime memory ops under `/memory`

- GET `/memories?limit={n}&cursor={cursor}&q={prefix}&fields=...`
- GET `/memories/{memory_id}`
- PUT `/memories/{memory_id}` — idempotent upsert (supports `If-Match`)
- DELETE `/memories/{memory_id}` — 204
  - Note: runtime memory operations remain under `/memory/...` as implemented today.

Project

- GET `/project` — get current project configuration
- PUT `/project` — upsert current project configuration (supports `If-Match`); 201 on first create, otherwise 200/204

Notes

- Query `project=` semantics remain available; default project comes from app state.
- Use a single idempotent `PUT` per resource for create-or-update by ID; `Location` set on 201.

Agents/Tools/Tasks (Phase 2):

- Child resources (agents, tools, tasks) are independent first‑class resources with top‑level CRUD and continue to have nested read views under `/workflows/{workflow_id}` for DX.
- Referential integrity rules apply:
  - A child resource referenced by any workflow cannot be hard‑deleted; the API returns `409 Conflict` with Problem Details listing referencing workflow IDs.
  - Soft delete may be introduced later (out of scope here).
- When fetching a workflow, the default representation includes child references by ID; clients may request expanded embedded children via `expand=tasks,agents,tools`.

Models/Schemas/Memory/Project (Phase 2):

- Promote to first-class endpoints similar to workflows. Endpoints: `/project`, `/models`, `/schemas`, `/memories` with CRUD mapped to per‑resource UC (`engine/{resource}/uc`) backed by the shared `engine/resources` store. Admin provenance remains under `/admin/meta/...`.

## Behaviors to Reuse from `/resources` (must‑haves)

1. Storage & Validation

- Always use per‑resource UC (`engine/{resource}/uc`) to perform create/upsert/get/list/delete per type. These UC call the shared `engine/resources` store interfaces.
- Continue using `logger.FromContext(ctx)` and `config.FromContext(ctx)` inside UC paths (already enforced). No global singletons.

2. ETag & If‑Match (optimistic concurrency)

- For upserts, read `If-Match` and perform a strong comparison only; weak validators are not accepted for `If-Match`.
- On success, set `ETag` header to the new strong ETag.
- On mismatch, return `412 Precondition Failed` with Problem Details.

3. Provenance metadata (`WriteMeta`)

- Preserve meta writes on create/upsert with `source=api` and `updated_by` derived from user context when available.

4. Project scoping

- Preserve `?project=` override with fallback to `state.ProjectConfig.Name`.

5. Location & Link headers

- On create, set `Location` to the canonical resource URL (e.g., `/api/v0/workflows/{workflow_id}`).
- For list endpoints with additional pages available, include `Link` headers with `rel="next"`/`rel="prev"`.

## Design Decisions

- Greenfield: Remove `/resources` router entirely. Update Swagger and tests. No shims.
- Unify reads: Rework existing GET endpoints in workflow/agent/tool/task routers to read via per‑resource UC to ensure store parity (no silent divergence from app state).
- Route scoping & middleware: Keep existing auth middleware; leverage route grouping for `/workflows` and nested executions.

## Implementation Plan

Phase 0 — Prep & Validation

- [x] Run `make lint` and `make test` to get a clean baseline.
- [ ] Confirm Swagger generation pipeline and tags are updated from main.go tags.

Phase 1 — Workflow writes

1. Add handlers to `engine/workflow/router`:
   - `PUT /workflows/{workflow_id}` (idempotent upsert)
   - `DELETE /workflows/{workflow_id}` (204 on success)
     Each handler MUST:

- Use `router.GetResourceStore(c)`
- Use per‑resource UC: `engine/workflow/uc.Upsert` (PUT) and `engine/workflow/uc.Delete` (DELETE)
- Bind body to a lightweight DTO for early validation, then convert to an internal struct/map for UC
- Read `If-Match` header; require strong ETag comparison (do not accept weak validators). On mismatch, return `412`.
- On create via PUT, set `Location: /api/v0/workflows/{id}` and return `201 Created`; on update, return `200 OK` with the full updated representation.
- Always set `ETag` on successful upsert.
- Respond using envelope helpers in `engine/infra/server/router/response.go`

2. Rewire workflow GETs to use store-backed per‑resource UC:
   - Replace current `appState.GetWorkflows()` calls with `engine/workflow/uc.Get`/`engine/workflow/uc.List` to ensure read-after-write consistency.
   - Implement cursor pagination for `GET /workflows` with `limit` and optional `cursor`; include `Link` headers and `page.next_cursor` in the body when applicable.

3. Swagger
   - Add annotations to the new handlers (tags: `workflows`, `executions`) with clear request/response.
   - Regenerate swagger via `make swagger`.
   - Document headers: `If-Match` (request), `ETag` and `Location` (response), `RateLimit-*`, and `Link` for pagination.
   - Use `application/problem+json` for error responses and document schema.

Phase 2 — Remove generic router

1. Delete `/resources` HTTP router registration from `engine/infra/server/reg_components.go`.
2. Remove `engine/resources/router/*` package.
3. Update tests:
   - Remove `test/integration/resources/*`.
   - Add workflow CRUD tests hitting the new endpoints.

Phase 3 — Broader resources (required)

- Promote `project`, `memory`, `model`, `schema`, and `mcp` to first-class endpoints with CRUD mapped to per‑resource UC.
- Create routers and UC packages:
  - `engine/project/router`, `engine/project/uc`
  - `engine/memoryconfig/router` (HTTP base `/memories`), `engine/memoryconfig/uc`
  - `engine/model/router`, `engine/model/uc`
  - `engine/schema/router`, `engine/schema/uc`
  - `engine/mcp/router`, `engine/mcp/uc`
- Keep `/admin/meta/...` as-is; adjust type mapping utilities accordingly.
- After feature parity, delete unused items from `engine/resources/uc/*`.

## API Shapes (Workflow)

1. Upsert (idempotent)

- PUT `/workflows/{workflow_id}`
- Headers: optional `If-Match: "<etag>"` (strong validators only)
- Body: full workflow object (ID may be omitted or must match path ID; enforced by UC).
- Responses:
  - 201 Created on create (sets `ETag` and `Location`)
  - 200 OK on update with the full representation (sets `ETag`)
  - 412 Precondition Failed on ETag mismatch

2. Delete

- DELETE `/workflows/{workflow_id}` → 204 No Content (idempotent; deleting missing returns 204 as well). If referenced by other entities, return 409 Conflict with Problem Details listing references.

3. Read

- GET `/workflows?limit={n}&cursor={cursor}` → `{ workflows: [...], page: { limit, next_cursor, prev_cursor } }` (+ `Link` header). `total` MAY be omitted under cursor pagination.
- GET `/workflows/{workflow_id}` → single item with `ETag` header. Default representation includes child references by ID; `expand=tasks,agents,tools` embeds child resources.
- GET `/executions/workflows` and GET `/workflows/{workflow_id}/executions` unchanged.

4. Execute (idempotency)

- POST `/workflows/{workflow_id}/executions`
- Headers: optional `Idempotency-Key: <opaque-token>` to safely retry without duplicate side effects.
- Responses: 202 Accepted (or 201 Created with execution location); duplicate key returns the original result.

Error Mapping

- Invalid payload / ID / type mismatch → 400 (Problem Details)
- Not found → 404 (Problem Details)
- ETag mismatch → 412 Precondition Failed (Problem Details)
- Referential integrity violation on delete → 409 Conflict (Problem Details)
- Store/unknown errors → 500 (Problem Details)

## Code Changes (high-level map)

- Add new handlers (workflow write): `engine/workflow/router` (new files for upsert/delete + shared helpers)
- Shared helpers:
  - Keep `projectFromQueryOrDefault` under `engine/infra/server/router`.
  - Remove/avoid `normalizeETag`; perform strict strong `If-Match` comparison in handlers/UC.
  - Add cursor pagination helpers and `Link` header construction utilities.
- Rewire workflow GETs to the store-backed UC and add pagination.
- Remove registration: edit `engine/infra/server/reg_components.go` to drop `resourcesrouter.Register(apiBase)`.
- Delete `engine/resources/router` package.
- Migrate business logic from `engine/resources/uc` into per‑resource UC packages. Keep `engine/resources/*store*` intact and used by all UC.
- Update Swagger tags in `main.go` if needed.

- Agents/Tools/Tasks: independent with referential integrity
  - Agents: add top-level list/get/put/delete in `engine/agent/router` and wire to UC/store; keep nested reads intact
  - Tools: add top-level list/get/put/delete in `engine/tool/router` and wire to UC/store; keep nested reads intact
  - Tasks: add a new top-level `engine/task/router` set for list/get/put/delete wired to UC/store; keep nested reads intact
  - Extend UC typed validators to include `ResourceTask` and implement reference checks on delete.

- MCP/Schemas/Models/Memories/Project
  - Create new routers: `engine/mcp/router`, `engine/schema/router`, `engine/model/router`, `engine/memoryconfig/router` (HTTP base `/memories`), and `engine/project/router`
  - Implement list/get/put/delete (pagination + If-Match) via per‑resource UC backed by the shared store
  - Use `/memories` as the base for memory resource configs to avoid conflict with runtime `/memory`

## API Shapes (Schemas)

1. Upsert (idempotent)

- PUT `/schemas/{schema_id}`
- Headers: optional `If-Match: "<etag>"` (strong validators only)
- Body: full schema object (ID may be omitted or must match path ID; enforced by UC)
- Responses:
  - 201 Created on create (sets `ETag` and `Location: /api/v0/schemas/{schema_id}`)
  - 200 OK on update with the full representation (sets `ETag`)
  - 412 Precondition Failed on ETag mismatch

2. Delete

- DELETE `/schemas/{schema_id}` → 204 No Content. If referenced by any resource (e.g., workflow), return 409 Conflict with references.

3. Read

- GET `/schemas?limit={n}&cursor={cursor}&q={prefix}&fields=...` → `{ schemas: [...], page: { limit, next_cursor } }` (+ `Link` header)
- GET `/schemas/{schema_id}` → single item with `ETag`

Error Mapping: same as Workflow.

## API Shapes (Models)

1. Upsert (idempotent)

- PUT `/models/{model_id}`
- Headers: optional `If-Match: "<etag>"` (strong validators only)
- Body: full model config (provider, name, parameters, etc.)
- Responses:
  - 201 Created on create (sets `ETag` and `Location: /api/v0/models/{model_id}`)
  - 200 OK on update with the full representation (sets `ETag`)
  - 412 Precondition Failed on ETag mismatch

2. Delete

- DELETE `/models/{model_id}` → 204 No Content. If referenced by any resource, return 409 Conflict with references.

3. Read

- GET `/models?limit={n}&cursor={cursor}&q={prefix}&fields=...` → `{ models: [...], page: { limit, next_cursor } }` (+ `Link` header)
- GET `/models/{model_id}` → single item with `ETag`

Error Mapping: same as Workflow.

## API Shapes (Memories)

Note: This covers memory resource configurations (`/memories`), not runtime memory operations (`/memory/...`).

1. Upsert (idempotent)

- PUT `/memories/{memory_id}`
- Headers: optional `If-Match: "<etag>"` (strong validators only)
- Body: memory config object
- Responses:
  - 201 Created on create (sets `ETag` and `Location: /api/v0/memories/{memory_id}`)
  - 200 OK on update with the full representation (sets `ETag`)
  - 412 Precondition Failed on ETag mismatch

2. Delete

- DELETE `/memories/{memory_id}` → 204 No Content. If referenced, return 409 Conflict with references.

3. Read

- GET `/memories?limit={n}&cursor={cursor}&q={prefix}&fields=...` → `{ memories: [...], page: { limit, next_cursor } }` (+ `Link` header)
- GET `/memories/{memory_id}` → single item with `ETag`

Error Mapping: same as Workflow.

## API Shapes (Project)

Project is a singleton resource within a scope.

1. Upsert (idempotent)

- PUT `/project`
- Headers: optional `If-Match: "<etag>"` (strong validators only)
- Body: full project configuration object
- Responses:
  - 201 Created on first create (sets `ETag` and `Location: /api/v0/project`)
  - 200 OK on update with the full representation (sets `ETag`)
  - 412 Precondition Failed on ETag mismatch

2. Read

- GET `/project` → single item with `ETag`

3. Delete

- DELETE is not exposed for the singleton project; resets happen via admin tooling (if ever). Returning `405 Method Not Allowed` is acceptable. Out of scope unless PRD requires it.

## Testing Plan

- Unit
  - New handler unit tests for upsert/delete with strong `If-Match` logic (412) and meta write success path.
  - Error paths: invalid payload, ID mismatch, unknown type, weak ETag rejection, stale/missing `If-Match`, store failures.
  - DTO binding tests (required fields, path/body ID alignment).
  - Problem Details conformance tests (structure, `application/problem+json`).

- Integration
  - Replace `test/integration/resources/http_*` with workflow + agents/tools/tasks CRUD tests hitting new endpoints.
  - Ensure `Location`, `ETag`, `RateLimit-*`, and `Link` headers are set correctly.
  - Verify read-after-write consistency via GET.
  - Verify cursor pagination semantics: `limit`, `cursor`, `Link` headers, and `page.next_cursor`.
  - Verify delete returns `409` when a resource is referenced by any workflow.
  - Verify `Idempotency-Key` deduplicates execution POSTs.

- Swagger
  - `make swagger` and validate updated docs; ensure tags and examples render.

## DTOs & Validation

- Define per-resource lightweight DTOs in each router package to enable early HTTP binding/validation.
- UC lives per resource in `engine/{resource}/uc` and should use typed inputs/outputs where helpful, delegating persistence to the shared store.
- Add missing typed validator for Task in `engine/task/uc/validators.go` (and equivalent per-resource validator locations).

## Risks & Mitigations

- Divergence between app state reads and store writes → Mitigation: unify reads to UC/store in Phase 1.
- Breaking API change → Greenfield policy allows removal; ensure docs/tests/CLI align before merge.
- ETag semantics → Align immediately to strong `If-Match` with 412 on mismatch.

## Rollout & Migration

- Single PR (greenfield) gated by passing `make lint` and `make test`.
- Update docs and examples to use new endpoints.
- Coordinate CLI commands (if any) that previously hit `/resources` (none currently identified, but re-check before merge).
- Document Problem Details, RateLimit headers, and Link header usage in Swagger and docs.

## References

- Internal
  - Store interface: `engine/resources/store.go`
  - UC layers: `engine/{workflow,agent,tool,task,model,schema,memoryconfig,project}/uc/*.go`
  - Current generic router: `engine/resources/router/*`
  - Workflow router: `engine/workflow/router/*`
  - Router helpers: `engine/infra/server/router/*`
  - Route bases: `engine/infra/server/routes/routes.go`

- External
  - Gin routing and groups (Context7 docs snapshot)
  - REST resource-specific endpoints & nested resources (various industry guides)
  - HTTP ETag/If-Match for optimistic concurrency (strong comparison; HTTP 412 on mismatch)
  - Web Linking for pagination (`Link` header)
  - Problem Details for HTTP APIs (`application/problem+json`)
  - RateLimit headers (in addition to legacy `X-RateLimit-*`)

## Acceptance Criteria

- Canonical CRUD for all resources:
  - Workflows: `/workflows`, `/workflows/{workflow_id}` (+ executions unchanged)
  - Tasks: `/tasks`, `/tasks/{task_id}` (nested read views remain)
  - Agents: `/agents`, `/agents/{agent_id}` (nested read views remain)
  - Tools: `/tools`, `/tools/{tool_id}` (nested read views remain)
  - MCPs: `/mcps`, `/mcps/{mcp_id}`
  - Schemas: `/schemas`, `/schemas/{schema_id}`
  - Models: `/models`, `/models/{model_id}`
  - Memory configs: `/memories`, `/memories/{memory_id}` (runtime memory stays under `/memory`)
  - Project: `/project`
- Upsert as a single idempotent `PUT /{resource}/{id}` across resources, using strong ETag comparison; 201 on create (with `Location`), 200 on update (with body + `ETag`).
- All list endpoints use cursor‑based pagination with `Link` headers; response body includes `page.next_cursor` when applicable.
- Error responses conform to Problem Details (`application/problem+json`).
- Rate limit responses include `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` headers.
- DELETE returns 204 No Content across resources; delete of referenced resources returns 409 Conflict with references.
- Workflows default to returning child references by ID; `expand=tasks,agents,tools` returns embedded child resources.
- All reads/writes go through per‑resource UC (`engine/{resource}/uc`) using the configured shared `ResourceStore`.
- All old `engine/resources/uc/*` entries not used anymore are deleted.
- Generic `/resources` router removed; Swagger no longer advertises it.
- `make lint` and `make test` pass.
