# Product Requirements Document (PRD)

## Overview

Replace the generic, polymorphic HTTP endpoints under `/api/v0/resources/{type}` with first‑class, resource‑specific APIs. Each domain resource (workflows, tasks, agents, tools, models, schemas, memories, project) exposes clear CRUD semantics, consistent headers, and predictable error envelopes. Business logic (use‑cases) is isolated per resource, while shared storage primitives remain centralized. This improves API discoverability, developer experience, and correctness by unifying reads and writes through the same persistence path.

## Goals

- Provide first‑class, resource‑specific CRUD endpoints (e.g., `/workflows`, `/tasks`, `/agents`, `/tools`, `/schemas`, `/models`, `/memories`, `/project`).
- Enforce optimistic concurrency with strong ETags and `If-Match` for idempotent upserts.
- Standardize pagination (cursor‑based + `Link` headers) and field shaping (`fields=`, `expand=`) where applicable. Cursor tokens encode the last seen resource identifier (keyset pagination) rather than numeric offsets to ensure stable traversal under concurrent writes.
- Standardize error responses using Problem Details (`application/problem+json`).
- Include rate limit response headers: `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`.
- Unify reads and writes via per‑resource UC backed by the shared `engine/resources` store to ensure read‑after‑write consistency.
- Remove the generic `/resources` HTTP router and its Swagger surface.

## User Stories

- As an API consumer, I can create or update a workflow via `PUT /api/v0/workflows/{workflow_id}` and receive a strong `ETag` to coordinate concurrent edits.
- As an API consumer, I can list workflows with cursor pagination and discover next/prev pages from `Link` headers.
- As an API consumer, I receive consistent Problem Details on all error responses, enabling simpler, resilient client handling.
- As a platform engineer, I can rely on per‑resource UC layers that consistently read/write through the shared store, eliminating divergence with in‑memory state.
- As a platform engineer, I can delete a resource safely and receive `409 Conflict` when referential integrity would be broken, with references described in the Problem Details.

## Core Features

1. Resource‑specific endpoints with CRUD semantics
   - Workflows (MVP): `GET /workflows`, `GET /workflows/{id}`, `PUT /workflows/{id}`, `DELETE /workflows/{id}`; executions routes remain as‑is.
   - Phase 2 resources: tasks, agents, tools, schemas, models, memories, project.
2. Idempotent upsert with strong ETag
   - `PUT /{resource}/{id}` accepts optional `If-Match` (strong only); returns `201 Created` on create (with `Location`) or `200 OK` on update, and always sets `ETag`.
3. Cursor‑based pagination for list endpoints

- Query `limit` (default 50, max 500) and opaque `cursor`; responses include `Link` headers and `page.next_cursor` when applicable. Cursors are keyset-based (`after`/`before` + workflow ID) so newly created or deleted workflows do not shift pagination results.

4. Standardized error envelopes
   - All 4xx/5xx return RFC 7807 Problem Details with `status`, `title`, and `detail` fields at minimum.
5. Consistent headers and field shaping
   - `RateLimit-*` headers on successful responses; `fields=` and `expand=` supported where applicable.
6. Read/write unification
   - All GET/PUT/DELETE handlers call per‑resource UC which delegates to the shared `engine/resources` store.

## User Experience

- Clear, discoverable endpoints organized by resource improve onboarding and client generation.
- Predictable pagination and error formats reduce bespoke client logic.
- Swagger/OpenAPI fully documents request/response bodies, headers (`If-Match`, `ETag`, `Location`, `Link`, `RateLimit-*`), and error shapes with examples.
- Accessibility of API docs: include exhaustive examples for success and error codes to support screen‑reader and code‑generation tooling.

## High‑Level Technical Constraints

- Greenfield policy: no backward compatibility required for legacy `/resources` routes; they are removed once resource endpoints reach parity.
- Retain existing auth/middleware model; authorization is out of PRD scope.
- Keep base path versioning as currently used (`/api/v0`).
- MUST use `logger.FromContext(ctx)` and `config.FromContext(ctx)` in all UC code paths; inherit request context.
- Persist via shared store interfaces (`engine/resources`); UC per resource is the orchestration layer.

## Non‑Goals (Out of Scope)

- Maintaining or shimming legacy `/resources` endpoints.
- Changing authentication/authorization models.
- Introducing GraphQL/gRPC.
- Soft‑delete or archival flows.

## Phased Rollout Plan

- MVP (Workflows)
  - Implement `PUT` and `DELETE` for `/workflows/{id}` with strong `If-Match` semantics and provenance metadata writes.
  - Rewire workflow `GET` endpoints to UC/store; add cursor pagination + `Link` headers and support `fields`/`expand`.
  - Document and publish via Swagger.
- Phase 2 (Other Resources)
  - Promote project, memories, models, schemas, agents, tools, tasks to top‑level CRUD with identical semantics (ETag, Problem Details, pagination).
  - Enforce referential integrity: delete returns `409 Conflict` when referenced.
- Phase 3 (Cleanup)
  - Remove generic `/resources` HTTP router and obsolete UC paths; update/clean tests and Swagger surface accordingly.

## Success Metrics

- All list endpoints implement cursor pagination with `Link` headers and include `page.next_cursor` when applicable.
- All successful write responses set `ETag`; `PUT` returns `201` with `Location` on create and `200` on update.
- `PUT` with stale or mismatched `If-Match` returns `412 Precondition Failed` with Problem Details.
- Error responses conform to RFC 7807; content type is `application/problem+json`.
- Rate limiting responses include `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`.
- Reads and writes for each resource go through per‑resource UC backed by the shared store.
- Generic `/resources` router removed from registration; Swagger no longer exposes it.
- `make lint` and `make test` pass.

## Risks and Mitigations

- Hidden consumers of `/resources` might exist internally → perform an organization‑wide search and publish findings; policy remains greenfield.
- Divergence between in‑memory reads and store writes → unify reads via UC/store from MVP onward.
- ETag/`If-Match` misuse by clients → provide precise Swagger examples and Problem Details responses.

## Open Questions

- Confirm no internal consumers of `/resources` remain; if found, coordinate a short deprecation.
- Confirm Swagger generation pipeline/tag organization and ensure header examples cover `If-Match`, `ETag`, `Location`, `Link`, and `RateLimit-*`.
- Confirm whether `total` is required in list responses under keyset pagination (tech spec allows omitting).

## Appendix

- See the technical specification at `tasks/prd-rest/_techspec.md` for detailed API shapes, DTO guidance, and acceptance criteria.
