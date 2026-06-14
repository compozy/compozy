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

## Shared Decisions
- Later tasks should consume `workspace.TaskRunMultipleModeEnqueued`, `workspace.TaskRunMultipleModeParallel`, and `TaskRunConfig.EffectiveRunMultipleMode()` instead of duplicating mode strings or default logic.
- V2 tasks should consume `workspace.DefaultRunMultipleParallelLimit` and `TaskRunConfig.EffectiveRunMultipleParallelLimit()` instead of hardcoding the default `2` or re-implementing the unset-limit fallback.

## Shared Learnings
- `parallel` is a valid configured value for V1, but execution tasks must still fall back to enqueued behavior until the daemon/worktree-backed parallel work lands.
- Multi-run slug input validation is centralized in `internal/core/tasks.ParseCommaSeparatedSlugs`; it trims entries, preserves order, rejects empty entries, and rejects duplicates.
- The local shell may export a stale `GOROOT=/Users/matheusbbarni/.local/go`; Go commands pass when run with `env -u GOROOT`.
- Multi-run parent events use `task.multi.*` event kinds with item statuses `queued`, `running`, `completed`, `failed`, and `canceled`; task_05 should consume the snapshot first and use events for incremental attach updates.
- CLI stream mode already renders `task.multi.*` parent queue events and returns exit code 1 when the parent ends failed/canceled/crashed; task_05 should avoid regressing this non-UI path when adding the tabbed TUI.
- Multi-run TUI tab navigation uses `[` and `]`; `tab` and `shift+tab` remain single-run pane focus controls inside the active child view.

## Open Risks

## Handoffs
- V2 task_03 CLI `--parallel-limit` precedence should fall back to `cfg.Tasks.Run.EffectiveRunMultipleParallelLimit()`; the daemon request / runtime-override surface (V2 task_02/06/08) carries the resolved positive limit. The config layer already rejects zero/negative at load; CLI must still reject zero/negative before daemon contact per the techspec.
- task_04 CLI wiring can call `tasks.ParseCommaSeparatedSlugs` for the `tasks run-multiple` positional argument, then use `cfg.Tasks.Run.EffectiveRunMultipleMode()` for the configured mode.
- task_04 can call daemon client `StartTaskRunMultiple(ctx, apicore.TaskRunMultipleRequest{...})`; task_05 can call `GetTaskRunMultipleSnapshot(ctx, parentRunID)` for parent attach state.
- Runtime override `run_id` is reserved for the parent multi-run request; children intentionally receive generated run IDs so a single request cannot duplicate child IDs.
- task_06 should document UI attach behavior: queued tabs appear before child runs exist, completed tabs remain navigable, `Close TUI` detaches, and `Stop Run` cancels the parent queue.
