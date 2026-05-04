# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Wire the task_04 daemon query layer into additive transport interfaces/services so later HTTP work can stay thin and handler-side joins stay out of scope.
- Success requires transport DTOs plus service methods for dashboard, workflow overview/board, spec, memory index/file, task detail, review detail, and richer run detail, with executable tests and clean `make verify`.

## Important Decisions
- Execute task_05 in the `daemon-web-ui` worktree, not the unrelated `unify-capability` checkout, because the required query layer and referenced transport files only exist here.
- Keep current transport methods for existing CLI/UDS callers and add new read-model methods rather than changing current signatures in place.
- Centralize query-to-transport mapping and typed error translation in daemon transport helpers instead of handlers.
- Build one shared `NewQueryService(...)` instance in `internal/daemon/host.go` and inject it into task/review transport services; keep `RunManager` as the daemon `RunService` by adding a `RunDetail` wrapper instead of introducing another run transport wrapper.

## Learnings
- `internal/daemon/query_service.go` already assembles every browser-facing read model required by task_05; the missing seam is transport exposure, not data assembly.
- Current transport gaps are concentrated in `internal/api/core/interfaces.go`, `internal/daemon/task_transport_service.go`, and `internal/daemon/review_exec_transport_service.go`.
- Expanding shared transport interfaces also requires updating downstream test doubles outside the daemon package, including the shared API handler fakes and the `pkg/compozy/runs` integration transport stub.
- The in-process CLI run stream adapter used by daemon-backed tests needs `Close()` to wait for the forwarder goroutine to stop; otherwise one more item can race through after close.

## Files / Surfaces
- `internal/api/core/handlers_service_errors_test.go`
- `internal/api/core/handlers_smoke_test.go`
- `internal/api/core/handlers_test.go`
- `internal/api/core/interfaces.go`
- `internal/api/httpapi/transport_integration_test.go`
- `internal/cli/daemon_exec_test_helpers_test.go`
- `internal/daemon/host.go`
- `internal/daemon/run_manager.go`
- `internal/daemon/task_transport_service.go`
- `internal/daemon/review_exec_transport_service.go`
- `internal/daemon/transport_read_models_test.go`
- `internal/daemon/transport_mappers.go`
- `pkg/compozy/runs/integration_test.go`

## Errors / Corrections
- Initial repo cwd pointed at a different branch/worktree (`unify-capability`) that does not contain task_04 query files; corrected by moving execution to `/Users/pedronauck/Dev/compozy/_worktrees/daemon-web-ui`.
- Fresh `make verify` surfaced two verification-only corrections after the transport work landed:
  - `pkg/compozy/runs/integration_test.go` needed a `RunDetail` test-double implementation to satisfy the expanded `RunService` interface.
  - `internal/cli/daemon_exec_test_helpers_test.go` had a real close-handshake race where the forwarder goroutine could outlive `Close()` and deliver an item after shutdown.
- The transport read-model fixture needs canonical path comparison on macOS temp dirs because the daemon resolves `/var/...` workspaces to `/private/var/...` when registering them.

## Ready for Next Run
- Task_05 is implemented and verified.
- Fresh evidence:
  - `go test ./internal/daemon`
  - `go test ./internal/api/core ./internal/api/httpapi`
  - `go test ./pkg/compozy/runs`
  - `go test -coverprofile=/tmp/daemon-task05.cover ./internal/daemon` => `80.4%`
  - `make verify`
- Tracking files still need to stay out of the automatic commit unless explicitly staged by policy.
