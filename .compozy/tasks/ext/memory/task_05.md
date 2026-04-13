# Task Memory: task_05.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement the hook dispatcher and Host API router glue layers for executable extensions, with deterministic hook ordering, capability enforcement, audit logging, and tests that cover dispatcher/router behavior at >=80% package coverage.

## Important Decisions
- Added a run-scoped `Registry` and `RuntimeExtension` model so the dispatcher and router share the same runtime session metadata instead of each carrying separate extension maps.
- Kept transport concerns behind `ExtensionCaller`, which only exposes request/response dispatch and leaves subprocess wiring to task 08.
- Implemented mutable hook chaining with top-level JSON patch merge that preserves the input's concrete Go type when possible, so downstream runtime code can keep using structs instead of being forced into `map[string]any`.
- `HostAPIRouter.RegisterService` accepts either `tasks` or `host.tasks` and normalizes both to the same namespace, while `Handle` still routes only `host.<namespace>.<verb>` methods.
- Because manifest hooks do not yet carry an explicit declaration name, `execute_hook` requests currently use the canonical hook event string as `hook.name`.

## Learnings
- Package coverage for `internal/core/extension` initially landed at `79.5%`; small helper-branch tests around registry/runtime helpers and JSON patch decoding were enough to lift the package to `80.8%`.
- The router needs to emit JSON-RPC errors for both lifecycle gating (`not_initialized`, `shutdown_in_progress`) and namespace routing (`method_not_found`) before task 06 plugs in real service handlers.

## Files / Surfaces
- `internal/core/extension/chain.go`
- `internal/core/extension/dispatcher.go`
- `internal/core/extension/host_api.go`
- `internal/core/extension/host_api_errors.go`
- `internal/core/extension/dispatcher_test.go`
- `internal/core/extension/host_api_test.go`

## Errors / Corrections
- `make verify` first failed on one `lll` lint violation in `internal/core/extension/host_api.go`; reformatted the function type declaration and reran the full pipeline successfully.

## Ready for Next Run
- Task 06 can register typed service handlers against `HostAPIRouter`.
- Task 08 can populate `Registry` entries with real `ExtensionCaller` transports and lifecycle state transitions without changing the dispatcher/router APIs.
