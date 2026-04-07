# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Completed task_05 by threading a per-run journal and event bus through prepare/execute/logging, replacing executor/logging `uiCh` writes with journal submissions, and covering post-execution task/review/provider events with bus + `events.jsonl` tests.

## Important Decisions
- Task_05 builds on the existing `internal/core/run/journal/` package from task_03 instead of re-implementing journal behavior.
- The task scope includes run lifecycle and shutdown events because the explicit task tests require `run.started`, terminal run events, and `shutdown.*` emissions in addition to executor/logging/session/post-success events.
- Exec mode now persists canonical runtime events through the shared journal while keeping the existing stdout JSON projection for CLI consumers.

## Learnings
- The provider abstraction does not currently expose transport-level status codes, so provider lifecycle payloads may need a best-effort status representation when the underlying implementation cannot surface one.
- Targeted tests exposed two production bugs during the refactor: shutdown termination events were losing the forced flag because state mutated too early, and lifecycle helpers assumed `execCtx.cfg` was always non-nil in tests and fallback paths.

## Files / Surfaces
- `.codex/CONTINUITY-executor-events.md`
- `internal/core/model/model.go`
- `internal/core/plan/prepare.go`
- `internal/core/kernel/deps.go`
- `internal/core/api.go`
- `internal/core/kernel/handlers.go`
- `internal/core/run/events.go`
- `internal/core/run/types.go`
- `internal/core/run/command_io.go`
- `internal/core/run/execution.go`
- `internal/core/run/logging.go`
- `internal/core/run/exec_flow.go`
- `internal/core/run/journal/journal.go`
- `internal/core/run/execution_test.go`
- `internal/core/run/execution_acp_test.go`
- `internal/core/run/execution_acp_integration_test.go`
- `internal/core/run/execution_ui_test.go`
- `internal/core/run/logging_test.go`
- `internal/core/kernel/deps_test.go`
- `internal/core/plan/prepare_test.go`

## Errors / Corrections
- Corrected stale assumption: `internal/core/run/journal/` is present in this worktree, so task_05 should integrate with it rather than treating journal creation as missing prerequisite work.
- Corrected executor regressions found by the new tests instead of weakening coverage: forced shutdown termination now records the right flag, and runtime lifecycle helpers tolerate nil config in isolated paths.

## Ready for Next Run
- Task_06 can now consume the event bus as the only executor/logging source for UI updates; `uiCh` remains declared only for the temporary adapter boundary.
