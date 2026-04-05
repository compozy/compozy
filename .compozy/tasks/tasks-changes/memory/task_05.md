# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement task metadata flow for the timeline header and render a dynamic title/type badge plus right-aligned runtime provider/model in the TUI timeline panel.
- Validation target met: `make verify` passed after the TUI/model/test changes, and `go test ./internal/core/run -cover` reached 80.2%.

## Important Decisions
- `IssueEntry` was not extended for task title/type; `buildBatchJob` reparses the single PRD task file in PRD mode and copies `TaskEntry.Title` / `TaskType` into `model.Job`.
- The UI model now keeps a pointer to runtime `config` so the meta row can read `cfg.ide` / `cfg.model`; provider display uses `agent.DisplayName` exactly as required.
- The timeline panel keeps the legacy `SESSION.TIMELINE` fallback label when `taskTitle` is empty, but still renders the new right-aligned runtime fragment. This follows the explicit MUST requirements over the older checklist bullet that implied the entire old layout should remain unchanged.
- A blank separator row is inserted only for the dynamic title header path so the titled layout gets the requested three-line header without needlessly changing the fallback header spacing.

## Learnings
- The run package needed additional focused UI helper tests beyond the task-specific rendering cases to lift package coverage over the required 80% threshold.
- The runtime provider/model fragment fits safely at `timelineMinWidth` (44) by truncating the left-side counter first.

## Files / Surfaces
- `internal/core/model/model.go`
- `internal/core/plan/prepare.go`
- `internal/core/plan/prepare_test.go`
- `internal/core/run/types.go`
- `internal/core/run/ui_model.go`
- `internal/core/run/ui_styles.go`
- `internal/core/run/ui_update.go`
- `internal/core/run/ui_update_test.go`
- `internal/core/run/ui_view.go`
- `internal/core/run/ui_view_test.go`

## Errors / Corrections
- Fixed a compile error in `buildBatchJob` by declaring `err` in the same scope as the new task metadata parse.
- Reworked a retry-meta assertion after the new right-aligned runtime fragment caused intentional left-side truncation at normal panel widths.
- Added missing shutdown-header test setup after an assertion accidentally exercised an incomplete-run status instead of the all-success branch.

## Ready for Next Run
- Task implementation, verification, and tracking updates are ready; remaining action is the local commit, with tracking/memory files intended to stay out of the auto-commit unless explicitly required.
