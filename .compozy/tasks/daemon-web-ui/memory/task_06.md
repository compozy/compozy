# Task Memory: task_06.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Add the browser-facing REST routes and handler wiring required by task_06 on the shared daemon transport stack, keep the existing root route families intact, align the checked-in OpenAPI contract, and cover the new surface with executable tests.

## Important Decisions
- Use the dedicated `daemon-web-ui` worktree as the implementation target because it already contains task_04/task_05 query and transport seams; the current cwd worktree does not match the task's route layout.
- Keep the new route/handler work inside `internal/api/core` and the shared HTTP/UDS registration points instead of adding transport-specific divergence.
- Align the checked-in browser contract to the current handler semantics in this checkout: browser reads use the `workspace` query parameter, browser task/review mutations keep `workspace` in JSON bodies, and stale workspace context is expressed as `412` `TransportError` responses instead of the earlier header-based draft contract.

## Learnings
- `internal/api/core/interfaces.go` already exposes `Dashboard`, `WorkflowOverview`, `TaskBoard`, `WorkflowSpec`, `WorkflowMemoryIndex/File`, `TaskDetail`, `ReviewDetail`, and `RunDetail`; task_06 is primarily missing the shared handler/route layer and contract artifact.
- The older task_02 artifact in the main checkout used a future header-based workspace contract and was not present in this worktree, so task_06 had to restore `openapi/compozy-daemon.json` here from the actual handler/route behavior instead of copying the stale draft verbatim.
- The new `internal/api/httpapi/openapi_contract_test.go` gives later tasks a route/contract parity guard so browser path drift is caught in Go tests without depending on frontend codegen being present in this worktree.

## Files / Surfaces
- `internal/api/core/{routes.go,handlers.go,errors.go,internal_helpers_test.go,handlers_error_paths_test.go,handlers_service_errors_test.go,handlers_smoke_test.go}`
- `internal/api/httpapi/{routes.go,transport_integration_test.go,openapi_contract_test.go}`
- `internal/api/udsapi/routes.go`
- `openapi/compozy-daemon.json`

## Errors / Corrections
- Initial inspection against the wrong checkout (`unify-capability`) showed unrelated APIs and missing daemon-web-ui files; corrected by switching implementation work to `/Users/pedronauck/Dev/compozy/_worktrees/daemon-web-ui`.
- `make verify` initially failed on a `goconst` lint violation for repeated `schema_too_new` strings in `internal/api/core/errors.go`; corrected by introducing the shared `codeSchemaTooNew` constant and reusing it in helper tests before rerunning the full gate.

## Ready for Next Run
- Completed. Fresh verification evidence:
  - `go test ./internal/api/core`
  - `go test ./internal/api/httpapi`
  - `go test ./internal/api/httpapi -run 'TestBrowserOpenAPIContract(MatchesRegisteredBrowserRoutes|KeepsWorkspaceContextAndProblemSemantics)$' -count=1`
  - `go test ./internal/api/udsapi`
  - `make verify`
- Next tasks can rely on the additive browser routes, the checked-in `openapi/compozy-daemon.json`, and the Go-side contract parity test when they wire workspace/security middleware and frontend consumers.
