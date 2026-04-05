# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Deliver the final ACP UI adaptation by rendering typed content blocks in the Bubble Tea viewport, showing `model.Usage` summaries/meta, and removing the remaining legacy ACP-predecessor code.

## Important Decisions
- Render the viewport from `uiJob.blocks` instead of the old derived line buffers; keep `errBuffer` only as a fallback source for non-block session error lines.
- Keep plain-text log rendering in `logging.go` for file/stdout sinks, but give the TUI its own richer block-aware renderer in `ui_view.go`.
- Move `normalizeAddDirs`, `formatShellCommand`, and `formatShellArg` into `internal/core/agent/registry.go` so `internal/core/agent/ide.go` can be deleted cleanly.

## Learnings
- The task_02 ACP runtime plumbing had already migrated message types to `jobUpdateMsg`/`usageUpdateMsg`; task_03 mainly had to finish rendering and remove the remaining legacy files/tests.
- The UI integration assertion needed a taller test viewport so mixed block traces were validated against rendering logic instead of scroll clipping.
- The implementation diff was already present in the worktree when this run started; this pass verified the ACP UI/task_03 surface and aligned the task tracking files with the fresh evidence.

## Files / Surfaces
- `internal/core/agent/registry.go`
- `internal/core/agent/ide.go` (deleted)
- `internal/core/agent/legacy_command_test.go` (deleted)
- `internal/core/run/logging.go`
- `internal/core/run/logging_test.go`
- `internal/core/run/ui_styles.go`
- `internal/core/run/ui_update.go`
- `internal/core/run/ui_update_test.go`
- `internal/core/run/ui_view.go`
- `internal/core/run/ui_view_test.go`
- `.compozy/tasks/acp-integration/memory/{MEMORY.md,task_03.md}`
- `.compozy/tasks/acp-integration/{task_03.md,_tasks.md}`

## Errors / Corrections
- `make verify` initially failed on two `ui_view.go` lint findings (`lll` and `appendCombine`); fixed the formatting/append shape and reran the full pipeline successfully.
- Task tracking was still marked pending even though the ACP UI worktree already satisfied the task requirements; this run reran the package-level validation plus `make verify` and then updated tracking to match the verified state.

## Ready for Next Run
- Task 03 is implemented, verified with package-level ACP/UI tests plus `make verify`, and ready for manual diff review without an automatic commit.
