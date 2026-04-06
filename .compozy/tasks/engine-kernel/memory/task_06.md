# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Decouple the PRD-task TUI from executor-owned `uiCh` wiring by translating bus events back into the existing internal `uiMsg` contract inside `ui_model.go`.

## Important Decisions
- Kept `ui_update.go` and `ui_view.go` unchanged; the adapter reconstructs the same `uiMsg` values and `SessionViewSnapshot` state the model already expects.
- Made the adapter stateful per UI session via `uiEventTranslator` so `session.update` events rebuild cumulative transcript snapshots instead of treating each update in isolation.
- Removed the dead `uiCh` plumbing from executor, session setup, logging, and exec helpers rather than preserving an unused compatibility seam.

## Learnings
- `job.failed` must fan out into both `jobFailureMsg` and a synthetic terminal `jobFinishedMsg` so the TUI preserves failure summaries and failed-count/state updates with the unchanged Bubble Tea model.
- The run-package race suite already had a real shared-stdio test hazard: `captureExecuteStreams` swaps process-global stdio, so tests that read `os.Stderr` directly must not run in parallel.

## Files / Surfaces
- `internal/core/run/ui_model.go`
- `internal/core/run/types.go`
- `internal/core/run/execution.go`
- `internal/core/run/command_io.go`
- `internal/core/run/logging.go`
- `internal/core/run/exec_flow.go`
- `internal/core/run/ui_adapter_test.go`
- `internal/core/run/execution_ui_test.go`
- `internal/core/run/execution_acp_test.go`
- `internal/core/run/execution_acp_integration_test.go`
- `internal/core/run/logging_test.go`
- `internal/core/run/ui_model_test.go`
- `internal/core/run/preflight_test.go`

## Errors / Corrections
- `make verify` initially failed on `ui_model.go` for `gocritic` and `gocyclo`; fixed by splitting event translation into smaller domain helpers and removing a single-case switch.
- `go test -race ./internal/core/run` initially failed because `TestResolvePreflightStderrAndIsInteractiveHelpers` read `os.Stderr` in parallel with stdio capture tests; corrected by making that test serial.

## Ready for Next Run
- Task 06 implementation, focused validation, run-package race test, and full `make verify` are complete; remaining work is task tracking/commit only.
