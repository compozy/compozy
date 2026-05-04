# Task Memory: task_12.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Adapt daemon-backed run observation so the Bubble Tea cockpit restores from `/runs/:run_id/snapshot`, then continues over `/runs/:run_id/stream`, while watch-only remains a separate non-TUI flow.
- Completed with clean `make verify`, focused CLI/UI/daemon tests, and task-surface coverage above 80% for the new remote observation packages.

## Important Decisions
- Keep the existing cockpit rendering/update model and convert daemon snapshot + stream data back into the existing `uiMsg` flow instead of redesigning the UI.
- Enrich snapshot hydration enough to restore stable per-job metadata before live stream subscription; the current persisted `job_state.summary_json` is too thin after later lifecycle updates overwrite queued metadata.
- Keep `brainstorming` intentionally skipped because task `12` already has an approved techspec/ADR-backed design and this run is governed by `cy-execute-task`.
- Keep watch-only and full TUI attach as separate CLI flows over the same snapshot + stream contract instead of trying to multiplex both UX modes through one command path.

## Learnings
- `tasks run` currently resolves daemon attach mode but only starts the run and prints a summary line.
- `runs` currently exposes only `purge`; attach/watch commands still need to be added.
- The daemon transport already provides `/runs/:run_id/snapshot`, cursor-based `/events`, and SSE `/stream` with heartbeat and overflow handling.
- The current TUI only hydrates from the in-process event bus; remote attach needs a bridge from daemon snapshot/SSE back into the existing `jobQueued/jobUpdate/jobFinished/...` message flow.
- Dense snapshot reconstruction had to come from durable run history and token-usage projections, not only the latest persisted `job_state.summary_json`, to restore queued metadata, transcript snapshots, usage, and terminal job state faithfully.
- Final focused coverage after the finished implementation remained above the required threshold: `internal/core/run/ui` at `81.2%` and `pkg/compozy/runs` at `81.1%`.

## Files / Surfaces
- `internal/daemon/run_manager.go`
- `internal/api/{core,client}`
- `internal/core/run/ui/*`
- `internal/cli/{daemon_commands.go,runs.go,root_command_execution_test.go,daemon_commands_test.go}`
- `pkg/compozy/runs/watch.go`
- `internal/daemon/run_snapshot.go`
- `internal/cli/run_observe.go`
- `pkg/compozy/runs/remote_watch.go`

## Errors / Corrections
- Initial `make verify` failures after implementation were lint-only:
  - extracted task-run attach/watch helpers in `internal/cli/daemon_commands.go`
  - split run/job/session watch rendering helpers in `internal/cli/run_observe.go`
  - split remote stream loops and snapshot bootstrap helpers in `internal/core/run/ui/remote.go` and `pkg/compozy/runs/remote_watch.go`
  - removed unused `error` return values from pure snapshot/render helpers

## Ready for Next Run
- Completed. Local code commit: `0a8c0c9` (`feat: add daemon-backed run attach and watch`).
- Follow-on daemon tasks can build on `internal/api/client/runs.go`, `internal/core/run/ui.AttachRemote`, `internal/cli/run_observe.go`, and `pkg/compozy/runs.WatchRemote` for daemon-backed observation instead of reintroducing local event sourcing.
