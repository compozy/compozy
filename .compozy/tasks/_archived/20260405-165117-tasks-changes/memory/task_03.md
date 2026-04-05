# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Implemented the `compozy migrate` v1→v2 pass, chained it after legacy→v1 for task files, migrated `.compozy/tasks/acp-integration/` to schema v2, and verified the repo with `compozy validate-tasks`, targeted tests, `go vet`, and `make verify`.

## Important Decisions

- Migration will resolve the workspace task type registry so remapping and fix-prompt output use the same allowlist as `validate-tasks`.
- Type remap logic will live in `internal/core/tasks/` instead of being duplicated in CLI and migration code.
- Existing unrelated dirty files in the worktree will be left untouched.
- Migration scanning now skips workflow `memory/` directories and ignores task-workflow `_meta.md`; only review-round `_meta.md` under `reviews-NNN` is treated as review metadata.
- ACP fixture task types were finalized as `backend`, `backend`, and `frontend` for tasks 1, 2, and 3 respectively.

## Learnings

- The migrate command needs workspace config access, not just the target path, because second-chance type matching and follow-up prompts depend on the resolved task type registry.
- The ACP fixture directory originally failed migration because the scanner descended into `memory/task_*.md` and misread the workflow `_meta.md`; fixing scanner scope was required before the task files could be migrated in-place.

## Files / Surfaces

- `internal/core/migrate.go`
- `internal/core/migrate_test.go`
- `internal/core/api.go`
- `internal/cli/root.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/migrate_command_test.go`
- `internal/core/tasks/`
- `.compozy/tasks/acp-integration/task_01.md`
- `.compozy/tasks/acp-integration/task_02.md`
- `.compozy/tasks/acp-integration/task_03.md`

## Errors / Corrections

- Corrected a scanner bug exposed by the required fixture migration: workflow `memory/` notes and task `_meta.md` were being treated as migratable artifacts, which aborted writes on valid task directories.

## Ready for Next Run

- Verified outcomes: `go run ./cmd/compozy validate-tasks --tasks-dir .compozy/tasks/acp-integration` returns `all tasks valid (3 scanned)`, `go run ./cmd/compozy migrate --tasks-dir .compozy/tasks/acp-integration --dry-run` reports `Migrated: 0`, `go vet ./...` passes, and `make verify` passes.
