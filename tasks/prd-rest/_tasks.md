# REST API Refactor (Resource-Specific) — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/workflow/router/register.go` - Workflow routes (GET/PUT/DELETE + executions)
- `engine/workflow/router/workflows.go` - Workflow handlers (GET list/get, PUT upsert, DELETE)
- `engine/workflow/uc/*` - Per‑workflow UC: upsert, delete, get, list
- `engine/infra/server/router/etag.go` - Strong ETag parsing (`If-Match`)
- `engine/infra/server/router/pagination.go` - Cursor encode/decode, `Link` headers
- `engine/infra/server/router/pagination.go` - Keyset (`after`/`before`) cursor encode/decode + `Link` headers
- `engine/infra/server/router/problem.go` - RFC 7807 responses
- `engine/infra/server/router/project.go` - Project resolution (`?project=` or default)
- `engine/infra/server/reg_components.go` - Component route registration (remove `/resources`)
- `engine/resources/router/*` - Legacy generic router (to delete)

### Integration Points

- `docs/swagger.yaml` / `docs/swagger.json` / `docs/docs.go` - OpenAPI surface
- `engine/infra/server/routes/routes.go` - Base URL helpers for `Location`

### Documentation Files

- `tasks/prd-rest/_prd.md` - PRD
- `tasks/prd-rest/_techspec.md` - Tech spec (source of truth for shapes)

## Tasks

- [ ] 1.0 Baseline & Swagger Pipeline Check
- [x] 2.0 Workflow PUT/DELETE endpoints (idempotent upsert, strong ETag)
- [x] 3.0 Workflow GET rewiring + cursor pagination + `Link`
- [x] 4.0 Router helpers: ETag, pagination, problem, project defaults
- [x] 5.0 Swagger updates for workflows (headers, examples, RFC7807)
- [x] 6.0 Remove generic `/resources` router (unregister + delete pkg)
- [x] 7.0 Promote Project/Schemas/Models/Memories CRUD endpoints
- [x] 8.0 Promote Agents/Tools/Tasks CRUD + referential integrity
- [ ] 9.0 Tests: unit + integration + OpenAPI validation gate
- [ ] 10.0 Rollout & Cleanup (delete legacy UC, docs/examples, org‑wide `/resources` scan)

## Execution Plan

- Critical Path: 1.0 → 2.0 → 3.0 → 5.0 → 6.0 → 7.0 → 8.0 → 9.0 → 10.0
- Parallel Track A (after 3.0): 5.0 (Swagger) can proceed alongside 6.0–8.0
- Parallel Track B (after 3.0): 9.0 test scaffolding and OpenAPI validation
- Parallel Track C (after 6.0): 10.0 docs/examples + deprecation scan

> Notes
>
> - Items 2.0–4.0 are already implemented locally per git status and code inspection; individual task files are marked `completed`.
