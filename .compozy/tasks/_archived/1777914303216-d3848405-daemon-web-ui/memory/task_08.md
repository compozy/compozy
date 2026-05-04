# Task Memory: task_08.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Embed the built SPA into the daemon binary and serve it from the existing localhost HTTP listener while preserving `/api`, keeping UDS API-only, and proving the behavior with unit/integration coverage.

## Important Decisions
- `internal/api/httpapi` owns SPA serving through an HTTP-only `NoRoute` fallback attached in `RegisterRoutes`; `core.RegisterRoutes` and `udsapi` remain API-only.
- Embedded bundle loading validates `dist/index.html` during `httpapi.New(...)` construction so a broken bundle contract fails early and clearly instead of at request time.
- Host runtime proof stays minimal: the existing `TestPrepareHostRuntimeStartsTransportsAndProbeReady` test now verifies `GET /` returns the embedded SPA through the live daemon HTTP listener.

## Learnings
- This worktree already had real `web/dist` build output checked in alongside the `.keep` placeholder, so `go:embed all:dist` could ship the actual bundle without widening task scope into build/CI wiring.
- `go test -cover ./internal/api/httpapi` reports `84.5%` coverage after the SPA-serving additions.

## Files / Surfaces
- `web/embed.go`
- `internal/api/httpapi/browser_middleware.go`
- `internal/api/httpapi/routes.go`
- `internal/api/httpapi/server.go`
- `internal/api/httpapi/static.go`
- `internal/api/httpapi/static_test.go`
- `internal/api/httpapi/transport_integration_test.go`
- `internal/daemon/host_runtime_test.go`

## Errors / Corrections
- Initial pre-change repro from `/tmp` failed because Go `internal/...` imports are module-scoped; reran the baseline from `.codex/tmp` inside the worktree and captured `GET / -> 404`.
- First `make verify` run failed on `noctx` and `goconst`; corrected the new tests to use context-aware HTTP requests and normalized repeated `"localhost"` literals into `localhostHost`.

## Ready for Next Run
- Task completed with clean `make verify`; later tasks can reuse the new HTTP/UDS transport tests as the daemon-served browser baseline.
