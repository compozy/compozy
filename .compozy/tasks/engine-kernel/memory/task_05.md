# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Thread a per-run journal and event bus through prepare/execute/logging, replace executor/logging `uiCh` writes with journal submissions, and emit post-execution task/review/provider events with tests covering bus and `events.jsonl`.

## Important Decisions
- The branch is missing `internal/core/run/journal/`, so task_05 will implement the minimum journal dependency needed for this task's executor integration while leaving task_03 tracking unchanged.

## Learnings
- `plan.Prepare` currently returns no journal handle and `SolvePreparation` has no journal or bus field.
- `internal/core/run/execution.go`, `internal/core/run/logging.go`, and `internal/core/run/exec_flow.go` still contain direct `uiCh` writes that block task_05 completion criteria.

## Files / Surfaces
- `internal/core/model/model.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/execution.go`
- `internal/core/run/logging.go`
- `internal/core/run/exec_flow.go`
- `internal/core/run/execution_test.go`
- `internal/core/run/execution_acp_test.go`
- `internal/core/run/execution_acp_integration_test.go`
- `internal/core/run/execution_ui_test.go`
- `internal/core/run/logging_test.go`

## Errors / Corrections
- Dependency gap: task_05 expects an `internal/core/run/journal/` package from task_03, but the package is absent in this worktree. Treating it as an implementation prerequisite instead of stopping, because task_05 cannot satisfy its own runtime contract without it.

## Ready for Next Run
- Continue by mapping the existing event payload contracts and runtime result emission, then implement journal creation/threading before migrating tests.
