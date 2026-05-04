# Task Memory: task_08.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Implement active-run workflow watchers for daemon-managed task/review runs.
- Reuse the existing sync ingestion path so manual Markdown edits update `global.db` during a run.
- Stop touched run/sync paths from recreating generated `_tasks.md` / `_meta.md` artifacts.
- Leave clear evidence through watcher/debounce/checkpoint/cleanup tests plus full verification.

## Important Decisions

- Use `internal/core/sync.go` as the canonical reconciliation path for watcher-triggered updates instead of custom file-specific patch logic.
- Tie watcher lifetime directly to daemon `RunManager` active-run ownership; teardown happens when the run terminates.
- Keep watcher scope to one workflow root and only react to Markdown artifacts inside that workflow tree.
- Run the initial workflow sync after run allocation but before watcher startup so duplicate run IDs fail on the canonical reservation path instead of inside pre-start sync.

## Learnings

- `artifact.updated` already exists in the runtime event model and `run.db` already projects it into `artifact_sync_log`, so watcher sync history can flow through the existing journal path.
- `internal/core/archive.go`, `internal/core/plan/prepare.go`, and executor/host write flows still depend on legacy metadata helpers today; they are part of the migration seam for this task.
- Current sync logic already removes workflow `_meta.md` plus only noncanonical `_tasks.md`; review-round `_meta.md` handling is still a live migration question.
- Executor tests that call `ReadTaskMeta` / `ReadRoundMeta` after touched run hooks now observe stale files by design; the correct assertion surface is `SnapshotTaskMeta` / `SnapshotRoundMeta`.
- The full repository gate passed after splitting `RunManager` startup and watcher loop helpers to satisfy `funlen` / `gocyclo` without changing behavior.

## Files / Surfaces

- `internal/daemon/run_manager.go`
- `internal/daemon/watchers.go` (new)
- `internal/daemon/watchers_test.go` (new)
- `internal/daemon/run_manager_test.go`
- `internal/daemon/purge_test.go`
- `internal/core/sync.go`
- `internal/core/sync_test.go`
- `internal/core/archive.go`
- `internal/core/archive_test.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/executor/{lifecycle.go,review_hooks.go}`
- `internal/core/tasks/store.go`
- `internal/core/reviews/store.go`
- `internal/store/{globaldb/sync.go,rundb/run_db.go}`
- `pkg/compozy/events/{event.go,kinds/extension.go}`

## Errors / Corrections

- Pre-change signal recorded: no run-scoped watcher implementation exists yet, task/review starts do not auto-sync, and several touched run/archive paths still recreate or require `_meta.md`.
- First focused test rerun exposed a real ordering bug: `syncWorkflowBeforeRun` ran before duplicate run ID reservation, so a losing concurrent start could fail from sync instead of `ErrRunAlreadyExists`.
- Focused executor failures were expectation drift, not production regressions: the touched task/review hooks no longer rewrite legacy metadata files and the tests had to stop reading those stale files.
- `make verify` initially failed on daemon lint complexity thresholds; the fix was to extract startup/default-resolution helpers and split the watcher loop into explicit debounce / flush helpers instead of weakening lint or changing behavior.

## Ready for Next Run

- Task is complete. Verification evidence:
- `go test ./internal/daemon ./internal/core/tasks ./internal/core/reviews ./internal/core/run/executor ./internal/core/plan ./internal/core/kernel ./internal/core/extension ./internal/store/globaldb ./internal/store/rundb -count=1`
- `go test -cover ./internal/daemon -count=1` -> `80.0%`
- `make verify`
- Remaining follow-up stays with `task_09`: archive eligibility still needs the planned DB-backed rewrite away from metadata-file checks.
