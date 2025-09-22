# Import/Export Per Resource — Design Review (2025-09-22)

## Objective

Greenfield implementation to replace the two generic admin endpoints with per‑resource collection actions, exposing `/export` and `/import` under each resource route. The legacy admin endpoints will be removed entirely.

Replace the generic admin endpoints with the following per‑resource routes:

- `/workflows/{import,export}`
- `/agents/{import,export}`
- `/tasks/{import,export}`
- `/tools/{import,export}`
- `/mcps/{import,export}`
- `/schemas/{import,export}`
- `/models/{import,export}`
- `/memories/{import,export}`
- `/project/{import,export}` (collection‑scoped for the single project document)

All new endpoints must inherit context correctly, return deterministic payloads, and update Swagger + CLI accordingly. Admin‑only gating is not required for these routes (see Security).

## Current State

- Admin routes: `engine/infra/server/reg_admin.go` registers:
  - `GET /admin/export-yaml` → `engine/infra/server/reg_admin_export.go`
  - `POST /admin/import-yaml` → `engine/infra/server/reg_admin_import.go`
- Importer: `engine/resources/importer/importer.go`
  - Supports: workflows, agents, tools, schemas, mcps, models, memories, project
  - Lacks: tasks
  - Internals allow per‑type apply (`applyForType`, upsert handlers)
- Exporter: `engine/resources/exporter/exporter.go`
  - Supports: workflows, agents, tools, schemas, mcps, models
  - Lacks: tasks, memories, project
- Resource routers exist for all resources (e.g., `engine/workflow/router/register.go`, `engine/agent/router/register.go`, etc.) but do not expose import/export endpoints.
- CLI: `cli/cmd/admin/admin.go` invokes the admin endpoints.
- Swagger references admin paths in `docs/swagger.*` and code annotations.

## Proposed Design

### 1) API Surface

- Add collection actions per resource:
  - Method: `POST`
  - Paths: `/{resource}/export` and `/{resource}/import`
  - Import supports `strategy` query (`seed_only` | `overwrite_conflicts`) mirroring current admin behavior.
- Responses mirror the current admin endpoints but scoped to a single resource type.
- Security: no special admin‑only gating (see Security below).

Project semantics: `/project/{import,export}` operate on the singleton project document. Payload counts should reflect a single entity (e.g., `written: 1` on export when present; `imported/overwritten: {project: 1}`), or 0 when absent.

Example responses:

- Export (200): `{ "data": { "written": 5 }, "message": "export completed" }`
- Import (200): `{ "data": { "imported": 3, "skipped": 2, "overwritten": 1, "strategy": "seed_only" }, "message": "import completed" }`

### 2) Security

- Admin‑only gating is NOT required for these new routes.
- Behavior: endpoints follow the global API auth configuration (e.g., `RequireAuth` if enabled). Do not attach any additional `RequireAdmin` middleware or checks.
- Rationale: import/export per resource is considered a standard operational capability; access control will be consistent with other resource CRUD endpoints.

### 3) Importer/Exporter Refactor

- Add type‑specific entry points so handlers don’t scan all directories:
  - `exporter.ExportTypeToDir(ctx, project, store, root, typ) (*Result, error)`
  - `importer.ImportTypeFromDir(ctx, project, store, root, strategy, updatedBy, typ) (*Result, error)`
- Extend type coverage for symmetry:
  - Importer: add tasks dir mapping and handler → `resources.ResourceTask` (via `taskuc.NewUpsert`).
  - Exporter: add `ResourceTask`, `ResourceMemory`, `ResourceProject` to `DirForType` and exported types list.
  - Directory names:
    - tasks: `tasks/`
    - memories: `memories/`
    - project: `project/`

### 4) Handlers (per resource)

- Register two new endpoints in each router package `register.go`:
  - `group.POST("/export", exportX)`
  - `group.POST("/import", importX)`
- Handler shape (pattern):
  1. `store, ok := router.GetResourceStore(c)`; `project := router.ProjectFromQueryOrDefault(c)`.
  2. Resolve project CWD/root from app state (same as admin handlers) → `extractProjectPaths` equivalent helper.
  3. Call importer/exporter type‑specific function with the corresponding `resources.ResourceType`.
  4. Build payload with only the scoped type counts.

### 5) Swagger

- Add `@Router /{resource}/export [post]` and `@Router /{resource}/import [post]` annotations to the new handlers, with `@Tags {resource}` and `@Security ApiKeyAuth` where applicable.
- Remove/replace admin endpoint annotations to avoid drift.
- Ensure responses use `router.Response` envelope and RFC7807 for errors, consistent with existing patterns.
- Verification: run `swag init` (or the repository’s docs generation) and grep `docs/*` to ensure no `/admin/(import|export)-yaml` paths remain.

### 6) CLI

- Deprecate/remove `cli/cmd/admin` subcommands.
- Add `import`/`export` subcommands under each resource (`cli/cmd/{workflows,agents,tasks,tools,mcps,schemas,models,memories,project}`) following current executor patterns. Include `--strategy` for import.

### 7) Backwards Compatibility

- Per project standards, backwards compatibility is not required during development. Remove admin endpoints and references entirely.

## Relevant Files to Change

- Router registration & handlers:
  - `engine/workflow/router/register.go`, `engine/workflow/router/*.go`
  - `engine/agent/router/register.go`, `engine/agent/router/*.go`
  - `engine/task/router/register.go`, `engine/task/router/*.go`
  - `engine/tool/router/register.go`, `engine/tool/router/*.go`
  - `engine/mcp/router/register.go`, `engine/mcp/router/*.go`
  - `engine/schema/router/register.go`, `engine/schema/router/*.go`
  - `engine/model/router/register.go`, `engine/model/router/*.go`
  - `engine/memoryconfig/router/register.go`, `engine/memoryconfig/router/*.go`
  - `engine/project/router/register.go`, `engine/project/router/*.go`
- Import/export core:
  - `engine/resources/exporter/exporter.go` (add `ExportTypeToDir`, extend types)
  - `engine/resources/importer/importer.go` (add `ImportTypeFromDir`, add tasks, map dirs)
- HTTP helpers: no new helpers required for admin gating.
- Remove legacy admin endpoints:
  - `engine/infra/server/reg_admin.go`
  - `engine/infra/server/reg_admin_export.go`
  - `engine/infra/server/reg_admin_import.go`
  - Update `engine/infra/server/register.go` accordingly
- CLI:
  - Remove: `cli/cmd/admin/*`
  - Add: `cli/cmd/{workflows,agents,tasks,tools,mcps,schemas,models,memories,project}/*`
- Swagger:
  - Update annotations in new handlers; regenerate `docs/*` as needed.

## Error Handling & Responses

- Use existing router helpers:
  - `router.RespondOK`, `router.RespondCreated`, `router.RespondWithError`, `core.RespondProblem` where applicable
  - Preserve strong ETag semantics where relevant (not applicable for import/export responses)
- Strategy validation for import: `seed_only` (default) | `overwrite_conflicts`; 400 on invalid param
- 401/403 via existing global auth middleware (no admin‑specific checks)

## Testing Strategy

- Unit
  - Importer: `ImportTypeFromDir` (happy path + invalid dir + strategy edge cases)
  - Exporter: `ExportTypeToDir` (writes deterministic YAML; verify filenames and counts)
  - Tasks: new upsert path via importer (ensures `WriteMeta` and ETag behavior)
- Router (integration‑style with gin engine)
  - For at least workflows and tools: project root resolution, payload shape
  - Auth behavior follows global settings; no admin‑only checks expected
- CLI
  - Validate command wiring and URL paths; strategy flag propagation
- Swagger
  - OpenAPI validation gate (ensure no stale `/admin/*` paths)

## Migration & Rollout

- Remove `/admin/import-yaml` and `/admin/export-yaml` routes and CLI in a clean sweep (greenfield approach).
- Communicate change in `CHANGELOG` (if applicable) and `docs` for operators: use per‑resource endpoints.

## Acceptance Criteria

- All listed resources expose working `/import` + `/export` endpoints without admin‑only gating.
- Import/Export symmetry includes tasks, memories, and project.
- CLI supports per‑resource import/export.
- Swagger updated; no `/admin/*` import/export in spec.
- `make lint` and `make test` pass.
- Exporter writes to `memories/` and `project/` directories; Importer and Exporter both handle `tasks/`.

## Legacy Admin Endpoints — Removal

- Remove the following and all references:
  - `engine/infra/server/reg_admin_export.go` (`/admin/export-yaml`)
  - `engine/infra/server/reg_admin_import.go` (`/admin/import-yaml`)
  - Any `@Router /admin/(export|import)-yaml` annotations in code/docs
- Remove CLI `cli/cmd/admin/*` and replace with per‑resource commands.

## Standards & Constraints

- Context inheritance: always use `c.Request.Context()` and `logger.FromContext(ctx)` / `config.FromContext(ctx)`; never `context.Background()` in runtime paths.
- Follow Go coding and testing standards in `.cursor/rules/*`.
- No workarounds in tests; adhere to testify patterns and subtest naming.
