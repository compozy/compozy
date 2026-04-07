# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Complete Task 02 by mechanically splitting the 12 oversized files named in `task_02.md` into the required same-package files with no API, signature, import-path, or behavior changes.
- Leave `internal/core/prompt/common.go` untouched in this phase.

## Important Decisions
- Use the task file, `_techspec.md`, `20260406-summary.md`, and the detailed analysis reports as the split map source of truth.
- Keep verification incremental by package while refactoring, then run full `make verify` before tracking updates and commit.
- Start with `internal/core/model/model.go`, then `internal/cli/root.go`, then the larger `internal/core/run` split set.

## Learnings
- The PRD directory for this task does not currently include an `adrs/` subdirectory.
- The worktree already has unrelated tracking edits in `_meta.md`, `_tasks.md`, `memory/task_01.md`, and `task_01.md`; leave them untouched.
- Pre-change signal: the 12 target files still total 9,160 lines and Task 02 remains pending.
- The `model` split verified cleanly with `go test ./internal/core/model/...`.
- The `cli` split verified cleanly with `go test ./internal/cli/...` after correcting missing file-local imports introduced by the split.
- The `run` package split verified cleanly with `go test ./internal/core/run/...` after correcting two missing file-local imports introduced during the split (`strings` in `ui_sidebar.go`, `tasks` in `execution.go`).
- The `agent` package split verified cleanly with `go test ./internal/core/agent/...`.
- The `workspace`, `events`, and `runs` splits verified cleanly with `go test ./internal/core/workspace/...`, `go test ./pkg/compozy/events/...`, and `go test ./pkg/compozy/runs/...`; the only post-split issue was one unused `encoding/json` import in `pkg/compozy/events/kinds/session.go`.
- To satisfy the task line-count criterion without changing behavior, `internal/core/run/execution.go` shed lifecycle helper functions into `lifecycle.go`, and `internal/core/agent/tool_call_input.go` moved ACP name helpers into `tool_call_name.go`; final sizes are 419 and 489 lines respectively.
- Final verification passed with `make verify` after all edits (`fmt`, `lint`, `956` tests, and `build`).

## Files / Surfaces
- `internal/core/model/model.go`
- `internal/core/model/constants.go`
- `internal/core/model/runtime_config.go`
- `internal/core/model/workspace_paths.go`
- `internal/core/model/artifacts.go`
- `internal/core/model/task_review.go`
- `internal/core/model/preparation.go`
- `internal/cli/root.go`
- `internal/cli/commands.go`
- `internal/cli/commands_simple.go`
- `internal/cli/dispatch_adapters.go`
- `internal/cli/state.go`
- `internal/cli/run.go`
- `internal/core/run/execution.go`
- `internal/core/run/shutdown.go`
- `internal/core/run/lifecycle.go`
- `internal/core/run/runner.go`
- `internal/core/run/session_exec.go`
- `internal/core/run/review_hooks.go`
- `internal/core/run/types.go`
- `internal/core/run/config.go`
- `internal/core/run/exec_types.go`
- `internal/core/run/ui_types.go`
- `internal/core/run/shutdown_types.go`
- `internal/core/run/logging.go`
- `internal/core/run/session_handler.go`
- `internal/core/run/render_blocks.go`
- `internal/core/run/buffers.go`
- `internal/core/run/ui_view.go`
- `internal/core/run/ui_summary.go`
- `internal/core/run/ui_sidebar.go`
- `internal/core/run/ui_timeline.go`
- `internal/core/agent/session.go`
- `internal/core/agent/acp_convert.go`
- `internal/core/agent/tool_call_format.go`
- `internal/core/agent/tool_call_name.go`
- `internal/core/agent/tool_call_input.go`
- `internal/core/agent/registry.go`
- `internal/core/agent/registry_specs.go`
- `internal/core/agent/registry_validate.go`
- `internal/core/agent/registry_launch.go`
- `internal/core/workspace/config.go`
- `internal/core/workspace/config_types.go`
- `internal/core/workspace/config_validate.go`
- `pkg/compozy/events/kinds/content_block.go`
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/replay.go`
- `pkg/compozy/runs/status.go`
- `pkg/compozy/events/kinds/session.go`

## Errors / Corrections
- Corrected missing `workspace` and `context` imports after the CLI file split; no logic changes were needed.
- Corrected missing `strings` and `tasks` imports after the `run` file splits; no logic changes were needed.
- Removed an unused `encoding/json` import after splitting `pkg/compozy/events/kinds/session.go`.

## Ready for Next Run
- Task 02 is complete: all required file-level splits landed, targeted files are under the 500-line limit, and final verification passed.
- Task 03 can proceed from the new split boundaries; `internal/core/prompt/common.go` remains intentionally untouched for the deferred parsing relocation.
