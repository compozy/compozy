# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- task_01 implemented the multi-run foundations: workspace config now supports `TaskRunConfig.RunMultipleMode` and `EffectiveRunMultipleMode()`, and comma-separated slug parsing lives in `internal/core/tasks.ParseCommaSeparatedSlugs`.
- task_02 added the daemon transport surface for multi-run parents: `POST /api/task-runs/multiple`, `GET /api/task-runs/multiple/:run_id/snapshot`, `contract.TaskRunMultipleRequest`, `contract.TaskRunMultipleSnapshot`, client methods `StartTaskRunMultiple` / `GetTaskRunMultipleSnapshot`, and OpenAPI/generated TS types.
- task_03 implemented the daemon-owned sequential `task_multi` parent coordinator: preflight happens before parent creation, children are regular `task` runs linked by `ParentRunID`, and snapshots replay parent queue events into ordered item state.
- task_04 wired `compozy tasks run-multiple [slugs]`: the CLI parses one comma-separated slug list, preflights all slugs before daemon contact, sends `TaskRunMultipleRequest` through the client, preserves `tasks run`, and supports non-UI detach/stream modes.
- task_05 added the tabbed daemon attach UI for `task_multi` parents: `ui.AttachRemoteMultiple` builds ordered tabs from the parent snapshot, keeps child `uiModel`/translator state isolated per slug, follows parent and child streams, and maps the quit dialog to queue detach/stop/cancel semantics.
- task_06 documented `tasks run-multiple`, `run_multiple_mode`, V1 `parallel` fallback, and queue-level TUI quit behavior in README, then added CLI integration coverage for comma-separated multi-run snapshot state and fallback-to-enqueued evidence.

> NOTE: bullets above use the OLD V1 task numbering. The bullets below use the CURRENT V2 (worktree-backed parallel) task numbering from `_tasks.md`; the two numbering schemes do not correspond.

- V2 task_01 added the bounded-parallel config foundation in `internal/core/workspace`: `TaskRunConfig.RunMultipleParallelLimit *int` (TOML `run_multiple_parallel_limit`), `EffectiveRunMultipleParallelLimit()` defaulting to `DefaultRunMultipleParallelLimit` (`2`), workspace-over-global merge mirroring `run_multiple_mode`, and validation rejecting zero/negative (shared `errMustBeGreaterThanZero` format). CLI and daemon scheduling are intentionally unchanged.
- V2 task_02 extended the multi-run transport contract additively: `contract.TaskRunMultipleRequest` now carries `ParallelLimit int` (`parallel_limit,omitempty`), and `contract.TaskRunMultipleItem` now carries `WorktreePath`/`BaseBranch`/`BaseCommit`/`WorktreeStatus` (`*,omitempty`). Client `StartTaskRunMultiple` forwards `ParallelLimit`; snapshot `Decode` already copies items so worktree fields round-trip. Handler `StartTaskRunMultiple` forwards `body.ParallelLimit` to `Tasks.StartRunMultiple`. OpenAPI (`openapi/compozy-daemon.json`: `parallel_limit` integer minimum 1; four item string fields) and generated `web/src/generated/compozy-openapi.d.ts` regenerated via `node scripts/codegen.mjs`. Routes unchanged; old payloads remain compatible. Event payload + snapshot reconstruction with worktree metadata remain task_04.
- V2 task_03 added the parallel CLI surface in `internal/cli`: `tasks run` now has `--parallel` and `--parallel-limit`, with mode precedence `--parallel` > config `run_multiple_mode` > `enqueued` and limit precedence `--parallel-limit` > config `run_multiple_parallel_limit` > `DefaultRunMultipleParallelLimit` (2). The V1 configured-parallel fallback message was removed; the CLI now forwards `parallel` (and the resolved positive `ParallelLimit`, only when mode is parallel) through `StartTaskRunMultiple`. `--parallel`/`--parallel-limit` without `--multiple`, and limit `<= 0`, are rejected before daemon contact. The daemon still rejects `parallel` (task_06), so end-to-end parallel runs fail at the daemon for now.

## Shared Decisions
- Later tasks should consume `workspace.TaskRunMultipleModeEnqueued`, `workspace.TaskRunMultipleModeParallel`, and `TaskRunConfig.EffectiveRunMultipleMode()` instead of duplicating mode strings or default logic.
- V2 tasks should consume `workspace.DefaultRunMultipleParallelLimit` and `TaskRunConfig.EffectiveRunMultipleParallelLimit()` instead of hardcoding the default `2` or re-implementing the unset-limit fallback.
- The multi-run transport contract is the single carrier for the V2 parallel surface: pass the resolved positive limit via `apicore.TaskRunMultipleRequest.ParallelLimit` and read child worktree metadata from `apicore.TaskRunMultipleItem` (`WorktreePath`/`BaseBranch`/`BaseCommit`/`WorktreeStatus`). The contract layer stays a pure DTO (no `workspace` import in source); resolution/defaults belong to CLI/daemon callers.
- The CLI sets `TaskRunMultipleRequest.ParallelLimit` only when the resolved mode is `parallel` (enqueued requests leave it 0/omitted). The daemon parallel scheduler (task_06/08) must therefore treat a missing/zero `ParallelLimit` on a parallel request as "use the configured/default limit" rather than failing — do not assume the CLI always populates it.

## Shared Learnings
- `parallel` is a valid configured value for V1, but execution tasks must still fall back to enqueued behavior until the daemon/worktree-backed parallel work lands.
- Multi-run slug input validation is centralized in `internal/core/tasks.ParseCommaSeparatedSlugs`; it trims entries, preserves order, rejects empty entries, and rejects duplicates.
- The local shell may export a stale `GOROOT=/Users/matheusbbarni/.local/go`; Go commands pass when run with `env -u GOROOT`.
- Multi-run parent events use `task.multi.*` event kinds with item statuses `queued`, `running`, `completed`, `failed`, and `canceled`; task_05 should consume the snapshot first and use events for incremental attach updates.
- CLI stream mode already renders `task.multi.*` parent queue events and returns exit code 1 when the parent ends failed/canceled/crashed; task_05 should avoid regressing this non-UI path when adding the tabbed TUI.
- Multi-run TUI tab navigation uses `[` and `]`; `tab` and `shift+tab` remain single-run pane focus controls inside the active child view.

## Open Risks

## Handoffs
- V2 task_03 DONE: CLI `--parallel`/`--parallel-limit` resolution + request wiring landed in `internal/cli/daemon_commands.go` (`resolveTaskRunMultipleMode`, `resolveTaskRunMultipleParallelLimit`, `rejectMultipleOnlyParallelFlags`). task_06 must make the daemon accept `parallel` (`internal/daemon/task_multi.go:resolveTaskMultiMode`) and consume `req.ParallelLimit`; then flip `TestTasksRunMultipleCommandInProcessParallelModeRejectedByDaemon` (currently asserts the daemon rejection) to assert a successful parallel run. The CLI already forwards `Mode="parallel"` and the resolved positive `ParallelLimit` (set only when mode is parallel).
- task_04 CLI wiring can call `tasks.ParseCommaSeparatedSlugs` for the `tasks run-multiple` positional argument, then use `cfg.Tasks.Run.EffectiveRunMultipleMode()` for the configured mode.
- task_04 can call daemon client `StartTaskRunMultiple(ctx, apicore.TaskRunMultipleRequest{...})`; task_05 can call `GetTaskRunMultipleSnapshot(ctx, parentRunID)` for parent attach state.
- Runtime override `run_id` is reserved for the parent multi-run request; children intentionally receive generated run IDs so a single request cannot duplicate child IDs.
- task_06 should document UI attach behavior: queued tabs appear before child runs exist, completed tabs remain navigable, `Close TUI` detaches, and `Stop Run` cancels the parent queue.
- V2 task_04 must extend `kinds.TaskRunMultiplePayload` and snapshot reconstruction with matching worktree fields (`worktree_path`/`base_branch`/`base_commit`/`worktree_status`) to mirror `contract.TaskRunMultipleItem`; the API contract, OpenAPI, and generated TS are already done in task_02, so only the event/payload + scheduler-built snapshot remain.
- V2 task_09 (TUI/`internal/core/run/ui/multi_remote.go`): `TaskRunMultipleItem` is now 128 bytes, so range over `Snapshot.Items` by index/pointer (gocritic `rangeValCopy` fires on value-copy loops); `newMultiRunTab` now takes `*apicore.TaskRunMultipleItem`.
