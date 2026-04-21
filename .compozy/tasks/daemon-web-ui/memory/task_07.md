# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Harden the browser-facing daemon API so browser requests use active-workspace headers, browser-only HTTP security, and explicit SSE semantics without regressing legacy query/body or UDS clients.

## Important Decisions
- Browser-only trust rules stay in `internal/api/httpapi`: Host validation, Origin validation, active-workspace header validation, and double-submit CSRF are applied only on the HTTP listener.
- Shared handlers resolve workspace context by preferring `X-Compozy-Workspace-ID` injected into request context, then falling back to legacy query/body workspace inputs for compatibility.
- `POST /api/sync` now treats a missing active workspace and missing explicit path as `workspace_context_missing` (`412`) while still accepting legacy body workspace/path targets.
- The canonical run SSE contract is `run.snapshot`, `run.event`, `run.heartbeat`, and `run.overflow`; daemon clients and `pkg/compozy/runs` keep legacy heartbeat/overflow decoder support so reconnect behavior does not regress.
- The checked-in browser OpenAPI artifact is updated in place and now advertises `X-Compozy-Workspace-ID`, relaxed browser mutation body schemas, and explicit `403` / `412` responses for browser security and workspace-context failures.

## Learnings
- The checked-in `openapi/compozy-daemon.json` artifact was the remaining contract drift point after the runtime changes landed; updating the contract test first made the JSON patch deterministic.
- Package-local tests in `internal/api/httpapi` are the fastest way to raise coverage for browser middleware and server helper code without bloating the integration suite.
- `internal/api/httpapi` coverage ended above the task target after helper coverage was added (`85.4%` in the local run), while `internal/api/core` (`85.2%`) and `pkg/compozy/runs` (`80.0%`) already satisfied the target on the changed backend surfaces.

## Files / Surfaces
- `internal/api/core/handlers.go`
- `internal/api/core/handlers_error_paths_test.go`
- `internal/api/core/handlers_service_errors_test.go`
- `internal/api/core/handlers_test.go`
- `internal/api/core/middleware.go`
- `internal/api/core/sse.go`
- `internal/api/core/sse_test.go`
- `internal/api/httpapi/browser_middleware.go`
- `internal/api/httpapi/browser_middleware_test.go`
- `internal/api/httpapi/openapi_contract_test.go`
- `internal/api/httpapi/server.go`
- `internal/api/httpapi/transport_integration_test.go`
- `internal/api/client/runs.go`
- `internal/api/client/runs_test.go`
- `pkg/compozy/runs/run.go`
- `pkg/compozy/runs/remote_watch.go`
- `pkg/compozy/runs/transport_test.go`
- `openapi/compozy-daemon.json`

## Errors / Corrections
- `POST /api/sync` initially still validated only the body before consulting the active-workspace context, which produced the old validation path instead of the required browser `412`; the handler and tests were corrected together.
- The initial OpenAPI artifact still advertised query-based workspace context and required browser mutation bodies to carry `workspace`; the contract test and checked-in JSON were updated to the new header-based semantics.
- `internal/api/httpapi` initially sat below the coverage target (`78.2%`) after the feature work; targeted helper tests raised it above the gate without expanding the task scope into unrelated transports.

## Ready for Next Run
- Run `make verify`, then update `task_07.md` / `_tasks.md` only if the full gate is clean and a self-review does not uncover new issues.
