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

---

## V2 (Worktree-Backed Parallel) — Extend Multi-Run API and Client Contracts

> The notes above are the OLD V1 task_02 (daemon transport bring-up). The notes below are the CURRENT V2 task_02 from `_tasks.md` (extend the existing contracts for parallel + worktree metadata). Different scope; same file number.

### Objective Snapshot
- Additively extend the existing multi-run request/snapshot contracts so callers pass the resolved parallel limit and snapshots carry per-child worktree metadata. Routes unchanged; old payloads stay compatible. Event payload + scheduler-built snapshot reconstruction are out of scope (task_04).

### Implemented
- `internal/api/contract/types.go`: `TaskRunMultipleRequest.ParallelLimit int` (`parallel_limit,omitempty`, after `Mode`); `TaskRunMultipleItem` gained `WorktreePath`/`BaseBranch`/`BaseCommit`/`WorktreeStatus` strings (`*,omitempty`). `apicore.*` are type aliases, so no change in `interfaces.go`.
- `internal/api/client/client.go` `StartTaskRunMultiple`: forwards `ParallelLimit`. `runs.go` `GetTaskRunMultipleSnapshot` needs no change — `TaskRunMultipleSnapshotResponse.Decode()` copies whole item structs.
- `internal/api/core/handlers.go` `StartTaskRunMultiple`: forwards `body.ParallelLimit` into `Tasks.StartRunMultiple`.
- `openapi/compozy-daemon.json`: `parallel_limit` (`type integer`, `minimum 1`) on request; four item string fields. Regenerated `web/src/generated/compozy-openapi.d.ts` via `node scripts/codegen.mjs`.

### Tests Added
- `internal/api/contract/contract_test.go`: `TestTaskRunMultipleContractCarriesParallelLimitAndWorktreeMetadata` — encode includes/omits `parallel_limit`; snapshot item round-trips worktree metadata in order; legacy item without worktree fields decodes.
- `internal/api/client/client_contract_test.go`: added "forward resolved parallel mode and limit" subtest; extended snapshot decode test with worktree metadata.
- `internal/api/core/handlers_smoke_test.go`: `TestStartTaskRunMultipleForwardsParallelLimitAndMode` via a `capturingTaskService` embedding `smokeTaskService`.
- `internal/api/httpapi/openapi_contract_test.go`: asserts `parallel_limit` on request schema and the four worktree fields on the item schema.

### Errors / Corrections
- `make lint` failed with gocritic `rangeValCopy` (item struct now exactly 128 bytes) at `internal/core/run/ui/multi_remote.go:172` and `internal/cli/root_command_execution_test.go:2816`. Root-cause fix: range by index; `newMultiRunTab` now takes `*apicore.TaskRunMultipleItem`; test loop uses `item := &snapshot.Items[i]`. No suppression.

### Verification
- `env -u GOROOT` for all Go/make commands. Passed: `make fmt`, `make lint` (0 issues), `make test` (3539 tests, 3 intentional skips), `make go-build`, `make frontend-typecheck` (codegen-check in sync).
