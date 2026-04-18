# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Rewrite `compozy sync` so it parses workflow artifacts and reconciles structured state into `global.db` instead of calling `tasks.RefreshTaskMeta`.
- Keep authored Markdown authoritative, remove sync-time `_tasks.md` / workflow `_meta.md` writes, and cover single-workflow + workspace-wide reconciliation with tests.

## Important Decisions
- Use the daemon techspec as the approved design baseline; no separate design artifact is needed for this implementation pass.
- Reuse existing task/review/memory readers where possible and add the new storage contract under `internal/store/globaldb`.
- Treat the current pre-change gap as `internal/core/sync.go` being metadata-only, with CLI/result wording that still assumes `_meta.md` refresh.
- Keep canonical authored `_tasks.md` files as artifact snapshots and remove only workflow `_meta.md` plus noncanonical/generated-looking `_tasks.md` during the one-time cleanup path.

## Learnings
- `globaldb` already has migrated tables for `artifact_snapshots`, `task_items`, `review_rounds`, `review_issues`, and `sync_checkpoints`; the missing piece is the reconciliation logic and the core sync integration.
- `internal/core/tasks/walker.go` and `internal/core/reviews/store.go` already provide ordered task/review entry reads that can feed projections without inventing new parsers.
- File-specific coverage for the new sync seams now exceeds the task target: `internal/core/sync.go` is at `84.1%` and `internal/store/globaldb/sync.go` is at `83.8%`.

## Files / Surfaces
- `internal/core/sync.go`
- `internal/core/sync_test.go`
- `internal/core/model/workflow_ops.go`
- `internal/store/globaldb/*`
- `internal/core/tasks/{parser.go,walker.go,store.go}`
- `internal/core/reviews/{parser.go,store.go}`
- `internal/core/memory/store.go`
- `internal/cli/commands_simple.go`
- `internal/cli/root.go`
- `internal/core/kernel/commands/workflow_sync.go`
- `test/public_api_test.go`

## Errors / Corrections
- `make verify` initially failed on a `funlen` hit in `ReconcileWorkflowSync` and a `gosec` warning for `os.ReadFile` inside `filepath.WalkDir`; the fix was to normalize the sync timestamp in a helper and read artifact bodies through `os.OpenRoot(...).ReadFile(...)`.

## Ready for Next Run
- Implementation, targeted coverage, and a final post-tracking `make verify` rerun are complete. Only task_08/task_09 follow-on work remains.
