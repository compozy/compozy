# Execution Endpoints — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/infra/server/router/idempotency.go` - API idempotency helper (new)
- `engine/agent/router/exec.go` - Direct agent execution (sync/async) and status (new)
- `engine/task/router/exec.go` - Direct task execution (sync/async) and status (new)
- `engine/workflow/router/execute_sync.go` - Synchronous workflow execution (new)

### Integration Points

- `engine/infra/server/router/helpers.go` - Param helpers, response helpers, worker readiness
- `engine/infra/server/reg_components.go` - Registers component route groups
- `engine/webhook/service.go` - Idempotency service reused by API helper
- `engine/task/uc/exec_task.go` - `uc.NewExecuteTask` used by direct exec
- `engine/infra/monitoring/*` - Metrics counters/timers wiring

### Documentation Files

- `tasks/prd-execs/_prd.md` - Product Requirements
- `tasks/prd-execs/_techspec.md` - Technical Specification
- `docs/content/docs/api/*` - OpenAPI-driven API docs (generated)

## Tasks

- [x] 1.0 Add execution status endpoints for agents and tasks
- [x] 2.0 Implement API idempotency helper (router wrapper)
- [x] 3.0 Implement direct Agent execution endpoints (sync/async)
- [x] 4.0 Implement direct Task execution endpoints (sync/async)
- [x] 5.0 Implement synchronous Workflow execution endpoint
- [x] 6.0 Wire metrics for execution endpoints
- [x] 7.0 Update OpenAPI and docs (examples, headers)
- [x] 8.0 Unit tests: validation, idempotency, responses
- [ ] 9.0 Integration tests: sync workflow + async flows
- [ ] 10.0 Developer guides and cURL examples

## Execution Plan

- Critical Path: 1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 6.0 → 7.0 → 8.0 → 9.0 → 10.0
- Parallel Track A: 3.0 ↔ 4.0 (after 1.0 and 2.0)
- Parallel Track B: 6.0 can proceed in parallel once 3.0/4.0 scaffolds exist

Notes:

- Keep `make lint` and `make test` green at every step.
- Always use `logger.FromContext(ctx)` and `config.FromContext(ctx)`; no globals and no `context.Background()` in runtime paths.
