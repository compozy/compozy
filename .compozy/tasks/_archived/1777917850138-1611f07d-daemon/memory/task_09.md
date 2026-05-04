# Task Memory: task_09.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Rewrite archive eligibility to use synced `global.db` task/review/run state instead of legacy `_meta.md` files while preserving the archive move semantics under the daemon model.
- Required outcomes: active-run conflict handling, `.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>` naming, deterministic workspace-wide skipped reporting, and required archive tests plus clean verification.

## Important Decisions
- Added archive-specific query/state helpers under `internal/store/globaldb/archive.go` so `internal/core/archive.go` no longer reparses task/review metadata files during archive.
- Single-workflow archive requests now return typed conflicts for unsynced/incomplete state or active runs; workspace-wide archive requests skip those workflows and record deterministic `SkippedPaths` / `SkippedReasons`.
- Archive flow now renames the workflow directory first, then marks the workflow row archived in `global.db`, and rolls the rename back if the DB update fails.
- Archive naming moved to `internal/core/model/workspace_paths.go` using `<timestamp-ms>-<shortid>-<slug>`.

## Learnings
- Synced `task_items`, `review_rounds`, and `runs` rows are sufficient to compute archive eligibility without reading any workflow or review `_meta.md` file.
- Reintroducing stale `_meta.md` files after sync is a useful regression test because it proves archive follows synced DB state instead of filesystem drift.
- Review resolution must still be re-synced before archive; editing review issue files alone is intentionally not enough.

## Files / Surfaces
- `internal/store/globaldb/archive.go`
- `internal/store/globaldb/archive_test.go`
- `internal/core/archive.go`
- `internal/core/archive_test.go`
- `internal/core/model/{workspace_paths.go,workflow_ops.go,model_test.go}`
- `internal/api/core/{errors.go,internal_helpers_test.go,handlers_service_errors_test.go}`
- `internal/cli/{commands_simple.go,archive_command_integration_test.go}`
- `test/public_api_test.go`

## Errors / Corrections
- Initial focused run failed because the new archive tests used `t.Parallel()` together with `t.Setenv()`. Removed `t.Parallel()` from those tests and reran the affected packages successfully.

## Ready for Next Run
- `make verify` passed after the final archive coverage/test updates.
- Required task tracking updates were applied and local commit `c920c7b` (`refactor: archive workflows from synced db state`) was created.
- No additional implementation follow-up is required for task_09.
