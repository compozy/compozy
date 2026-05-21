# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement the tabbed multi-run TUI attach path for daemon-owned `task_multi` parents: tabs for every requested slug, isolated child run UI state per tab, live parent/child updates, and queue-level quit dialog semantics.
- Preserve single-run `tasks run` UI behavior; scope is limited to multi-run parent attach and the CLI bridge that chooses it.

## Important Decisions
- Follow ADR-004 exactly: `Close TUI` detaches from the parent queue, `Stop Run` cancels the parent coordinator, and `Cancel` returns to the tabbed UI without side effects.
- Implemented the multi-run TUI as a wrapper around the existing single-run `uiModel` instead of changing the single-run layout; each tab owns its own child `uiModel` and `uiEventTranslator`.
- Chose `[` and `]` for tab navigation so `tab` / `shift+tab` continue to control existing pane focus inside the child run UI.
- CLI attach now detects parent snapshots with mode `task_multi` and routes UI/auto presentation to `ui.AttachRemoteMultiple`; single-run settled-before-attach behavior remains unchanged.

## Learnings
- Parent queue snapshots are the initial source of tab ordering and queued/not-started visibility; `task.multi.*` parent events are used for incremental tab status and child run discovery.
- Child streams should start once per discovered child run ID and route events back by slug, otherwise job indexes/transcripts can bleed across tabs.
- `Close TUI` must never call parent cancel from the CLI bridge; only the queue-level `Stop Run` quit action or context-owned cancellation should request `CancelRun` on the parent.

## Files / Surfaces
- Expected implementation surfaces: `internal/core/run/ui/*`, `internal/cli/run_observe.go`, and focused TUI/remote tests.
- Added `internal/core/run/ui/multi_remote.go` for the remote multi-run attach model/controller.
- Added `internal/core/run/ui/multi_remote_test.go` for tab rendering/state, quit behavior, and remote attach parent/child stream tests.
- Updated `internal/cli/run_observe.go` to route `task_multi` parent runs to the multi-run TUI attach session.
- Updated `internal/core/run/ui/update.go`, `view.go`, and `update_test.go` to centralize key literals required by lint while preserving single-run behavior.

## Errors / Corrections
- Initial full `make verify` failed on `gocyclo` in `attachRemoteCLIRunUI`; extracted request normalization and shared remote UI wait/cancel handling.
- Earlier lint also required key literal constants and a no-result parent-event helper; fixed before the passing full gate.

## Ready for Next Run
- Verified with `env -u GOROOT go test -timeout=60s ./internal/core/run/ui ./internal/cli`.
- Verified with `env -u GOROOT go test -coverprofile=/tmp/compozy-ui.cover ./internal/core/run/ui` at 80.2% statement coverage.
- Verified with `env -u GOROOT make lint`.
- Verified with `env -u GOROOT make verify`; full pipeline passed: frontend bootstrap/lint/typecheck/tests/build, Go fmt/lint/race tests/build, and frontend e2e.
