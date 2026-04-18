Goal (incl. success criteria):

- Complete daemon `task_09` by moving workflow archive eligibility from legacy metadata files to synced `global.db` state.
- Success requires DB-backed task/review/run eligibility checks, active-run conflict handling, `.compozy/tasks/_archived/<timestamp-ms>-<shortid>-<slug>` naming, deterministic workspace-wide archive reporting, required archive tests, and a clean `make verify`.

Constraints/Assumptions:

- Follow repo instructions from `AGENTS.md`, `CLAUDE.md`, daemon `_techspec.md`, `_tasks.md`, `task_09.md`, ADR-002, workflow memory, and the required `cy-workflow-memory` / `cy-execute-task` / `cy-final-verify` workflow.
- Required skills in use for this task: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`.
- The worktree already contains unrelated task-tracking and ledger edits; do not touch or revert unrelated changes.
- Keep scope tight to archive behavior, task memory, and required tracking updates.

Key decisions:

- Add archive-specific query/state helpers under `internal/store/globaldb/archive.go` instead of reparsing task/review files from `internal/core/archive.go`.
- Use synced `task_items`, `review_rounds`, and `runs` rows as the only eligibility source; missing sync state is treated as non-archivable.
- Treat single-workflow archive conflicts as errors and workspace-wide conflicts as deterministic skips with reasons.
- Rename the workflow directory first, then mark the workflow row archived, and roll the rename back if the DB update fails so filesystem and catalog state stay aligned.
- Move archive naming to `internal/core/model/workspace_paths.go` so the timestamp-ms + shortid format is shared and testable.

State:

- Completed after clean verification, required task tracking updates, and local commit `c920c7b` (`refactor: archive workflows from synced db state`).

Done:

- Read required instructions, daemon docs, workflow memory, and relevant daemon ledgers from tasks 07 and 08.
- Captured the pre-change signal: archive still depended on `tasks.RefreshTaskMeta`, review `_meta.md`, and the old timestamp-slug archive name.
- Added DB-backed archive eligibility/state APIs plus typed archive conflict errors in `internal/store/globaldb/archive.go` with unit coverage.
- Refactored `internal/core/archive.go` to use synced DB state, handle active-run conflicts, detect archived identities, and use timestamp-ms + shortid archive names.
- Added deterministic `SkippedPaths` reporting and updated archive CLI help/output to describe DB-backed semantics.
- Added or updated tests in `internal/core`, `internal/store/globaldb`, `internal/api/core`, `internal/cli`, and `test/public_api_test.go`.
- Ran focused tests successfully:
  - `go test ./internal/store/globaldb`
  - `go test ./internal/core`
  - `go test ./internal/api/core`
  - `go test ./internal/cli`
  - `go test ./test`
- Ran `make verify` successfully.
- Updated `.compozy/tasks/daemon/task_09.md` and `.compozy/tasks/daemon/_tasks.md` to completed.
- Created local commit `c920c7b` with the archive rewrite.

Now:

- Final handoff only.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None blocking so far.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-archive-db-state.md`
- `.compozy/tasks/daemon/{_techspec.md,_tasks.md,task_09.md,adrs/adr-002.md}`
- `.compozy/tasks/daemon/memory/{MEMORY.md,task_09.md}`
- `internal/core/{archive.go,archive_test.go,sync.go,sync_test.go}`
- `internal/core/model/{workspace_paths.go,workflow_ops.go,model_test.go}`
- `internal/store/globaldb/{archive.go,archive_test.go,registry.go,runs.go,sync.go}`
- `internal/api/core/{errors.go,internal_helpers_test.go,handlers_service_errors_test.go}`
- `internal/cli/{commands_simple.go,archive_command_integration_test.go}`
- `test/public_api_test.go`
- Commands:
- `go test ./internal/store/globaldb`
- `go test ./internal/core`
- `go test ./internal/api/core`
- `go test ./internal/cli`
- `go test ./test`
