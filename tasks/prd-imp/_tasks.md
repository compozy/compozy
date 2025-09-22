# Import/Export Per Resource — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/resources/exporter/exporter.go` — Add `ExportTypeToDir`, extend type coverage (tasks, memories, project)
- `engine/resources/importer/importer.go` — Add `ImportTypeFromDir`, add tasks support, dir mappings

### Resource Routers

- `engine/workflow/router/*` — Add POST `/workflows/{export,import}`
- `engine/agent/router/*` — Add POST `/agents/{export,import}`
- `engine/task/router/*` — Add POST `/tasks/{export,import}`
- `engine/tool/router/*` — Add POST `/tools/{export,import}`
- `engine/mcp/router/*` — Add POST `/mcps/{export,import}`
- `engine/schema/router/*` — Add POST `/schemas/{export,import}`
- `engine/model/router/*` — Add POST `/models/{export,import}`
- `engine/memoryconfig/router/*` — Add POST `/memories/{export,import}`
- `engine/project/router/*` — Add POST `/project/{export,import}`

### Deletions & CLI

- Remove admin endpoints: `engine/infra/server/reg_admin*.go`
- CLI: drop `cli/cmd/admin`, add per‑resource import/export commands

### Documentation

- Update Swagger annotations in new handlers; regenerate `docs/`

Note: All tasks MUST follow `.cursor/rules/go-coding-standards.mdc` and `.cursor/rules/test-standards.mdc`, inherit context correctly, use `logger.FromContext(ctx)` and `config.FromContext(ctx)`, and avoid global singletons.

## Tasks

- [ ] 1.0 Importer/Exporter symmetry + type‑specific functions
- [ ] 3.0 Resource routers: workflows/agents/tools/tasks
- [ ] 4.0 Resource routers: mcps/schemas/models/memories/project
- [ ] 5.0 Swagger updates and regeneration
- [ ] 6.0 CLI per‑resource import/export; remove admin
- [ ] 7.0 Remove admin routes and references
- [ ] 8.0 Tests: importer/exporter + router + CLI

## Execution Plan

- Critical Path: 1.0 → 3.0 → 4.0 → 5.0 → 8.0 → 6.0 → 7.0
- Parallel Track A: 3.0 and 4.0 can split by resource groups after 2.0
- Parallel Track B: 5.0 Swagger work can run alongside 3.0/4.0 once annotations start landing
