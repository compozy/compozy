# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Document `compozy tasks run-multiple` usage in README near the existing `tasks run` and config guidance.
- Add integration coverage proving comma-separated `tasks run-multiple alpha,beta` creates a daemon-owned parent queue whose snapshot contains both requested slugs.
- Cover the `run_multiple_mode = "parallel"` V1 fallback path with observable fallback messaging and enqueued execution evidence.

## Important Decisions
- Keep documentation in README only unless implementation shows generated command docs are also required.
- Reuse the existing in-process daemon CLI test harness rather than adding a separate external process or web smoke test.

## Learnings
- Shared memory says task_04 already centralized comma-separated slug parsing in `internal/core/tasks.ParseCommaSeparatedSlugs`.
- Shared memory says task_05 established queue TUI semantics: queued tabs exist before child runs, completed tabs remain navigable, `Close TUI` detaches, and `Stop Run` cancels the parent queue.
- The existing CLI command help already had `run-multiple` examples; task_06 added a focused help test to keep those examples and shared task-run flags covered.
- In-process CLI stream mode gives deterministic end-to-end evidence for parent queue events and snapshots after both dry-run child workflows complete.
- Focused CLI tests passed, and the full `make verify` gate passed after implementation and tracking updates.

## Files / Surfaces
- `README.md`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/daemon_exec_test_helpers_test.go`
- `internal/cli/root_test.go`
- `.compozy/tasks/multi-task-run/task_06.md`
- `.compozy/tasks/multi-task-run/_tasks.md`

## Errors / Corrections
- `rtk git diff -- ...` treated the path as a revision; use `rtk proxy git diff -- ...` for path-limited diffs.

## Ready for Next Run
- No known follow-up work for task_06.
