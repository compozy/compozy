# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Complete the Phase 0 quick-win refactors listed in `task_01.md` without expanding scope.

## Important Decisions

- Reused canonical implementations from `internal/core`, `internal/core/tasks`, and `internal/core/reviews` instead of keeping package-local duplicates.
- Kept the quick-win scope tight to the listed findings; skipped unrelated cleanup in other files even when nearby duplication existed.

## Learnings

- The duplicate kernel converter had already diverged from the canonical core clone path by omitting internal `groups`, so reusing the core helper also closes a live drift vector.
- Moving `plan/input.go` notices from stdout to `slog.Info` required test instrumentation to capture the default structured logger instead of standard output.
- Embedding shared event payload structs preserved access at runtime but required nested composite literals anywhere tests or producers instantiate those payloads directly.

## Files / Surfaces

- `internal/core/api.go`
- `internal/core/kernel/handlers.go`
- `internal/core/kernel/deps_test.go`
- `internal/core/tasks/store.go`
- `internal/core/reviews/store.go`
- `internal/core/plan/input.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/validation_form.go`
- `internal/core/run/validation_form_test.go`
- `internal/core/run/session_view_model.go`
- `internal/core/run/command_io.go`
- `internal/core/run/execution.go`
- `internal/core/run/exec_flow.go`

## Errors / Corrections

- Initial multi-file patch failed on a stale context in `handlers.go` and a mismatched test assertion string in `deps_test.go`; reapplied safely in smaller file-by-file chunks.

## Ready for Next Run

- Task implementation is complete.
- `make verify` passed cleanly after the final batch.
- Tracking files still need to remain out of the automatic code commit unless explicitly required.
