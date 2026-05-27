# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Add daemon API contract/client/handler/OpenAPI surface for starting a daemon-owned multi-run parent and reading its snapshot. Keep this task scoped to transport contracts; the coordinator implementation remains task_03.

## Important Decisions
- Treat ADR-002/003/004 as superseding ADR-001's earlier `tasks run` extension wording. This task uses `/api/task-runs/multiple` and preserves existing `StartTaskRun`.
- Keep task_02 limited to contracts/client/handlers. The real daemon coordinator remains task_03, so `transportTaskService` exposes the new methods but returns service-unavailable placeholders for now.
- Multi-run start and snapshot routes are workspace-context compatible like existing start routes; `workspace` remains optional in OpenAPI because the active workspace header can supply it.

## Learnings
- Pre-change signal: no `TaskRunMultipleRequest`, `TaskRunMultipleSnapshot`, or `/api/task-runs/multiple` route/client surface exists under `internal` or `openapi`.
- Updating `openapi/compozy-daemon.json` requires regenerating `web/src/generated/compozy-openapi.d.ts` with `bun run codegen`.
- The shell exports a stale `GOROOT=/Users/matheusbbarni/.local/go`; use `env -u GOROOT` for Go verification commands in this worktree.
- `RegisterRoutes` was already near the lint statement limit. Adding routes required splitting route registration into helper functions instead of suppressing `funlen`.
- To avoid stale turbo log replay from previous environment warnings, use `TURBO_FORCE=true` with `make verify` when clean verification output matters.

## Files / Surfaces
- `internal/api/contract`: multi-run request/snapshot types, route inventory, timeout and round-trip tests.
- `internal/api/core`: shared interface aliases, routes, handlers, smoke/contract/error-path stubs.
- `internal/api/client`: `StartTaskRunMultiple`, `GetTaskRunMultipleSnapshot`, client routing/encoding/nil-context tests.
- `internal/daemon/task_transport_service.go`: temporary multi-run service method stubs for task_03.
- `openapi/compozy-daemon.json` and `web/src/generated/compozy-openapi.d.ts`: daemon API schema and generated TS contract.

## Errors / Corrections
- First `make verify` failed because generated OpenAPI TS was stale; ran `bun run codegen`.
- Second `make verify` failed on `internal/api/core/routes.go` `funlen`; refactored route registration into focused helper functions.

## Ready for Next Run
- Clean verification command used for final gate: `env -u GOROOT -u NO_COLOR TURBO_FORCE=true make verify`.
