# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Completed the shared daemon transport layer for both UDS and localhost HTTP, with one route/resource model, request IDs, JSON error envelopes, and the SSE contract defined in `_techspec.md`.

## Important Decisions
- Use one shared handler core and thin HTTP/UDS wrappers so route behavior cannot drift by transport.
- Keep run/task/review/workspace/sync/exec behavior behind injected service interfaces; transport code should not embed future daemon manager logic.
- Keep HTTP server binding strict to `127.0.0.1`, persist the chosen ephemeral port through the daemon host seam, and chmod UDS sockets to `0600` after bind.
- Keep `StreamRun` transport-focused: cursor parsing, heartbeat, overflow, and error-frame semantics live in `internal/api/core`, while run persistence and fan-out stay behind `RunService`.

## Learnings
- `internal/api/core` can carry the full shared transport contract without coupling later daemon manager logic into the HTTP or UDS server packages.
- The repo lint contract requires context-aware request constructors in all transport tests and will reject the stream handler if SSE control flow is left monolithic.
- The shared handler/SSE package now reaches `85.0%` statement coverage, satisfying the task coverage target.

## Files / Surfaces
- `go.mod`
- `go.sum`
- `internal/daemon/boot.go`
- `internal/api/core/*`
- `internal/api/httpapi/*`
- `internal/api/udsapi/*`

## Errors / Corrections
- Initial transport tests left several `noctx` violations and the shared `StreamRun` handler over the repository `gocyclo` threshold; both were fixed by switching every request constructor to the context-aware forms and extracting the stream loop helpers without changing behavior.

## Ready for Next Run
- Task `04` is complete after `make verify` passed with the shared transport core, HTTP/UDS servers, parity tests, and SSE/request-ID/error-envelope contract in place.
