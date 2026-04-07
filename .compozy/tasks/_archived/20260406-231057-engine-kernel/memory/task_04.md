# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement task 04 by adding `internal/core/kernel/` and `internal/core/kernel/commands/` with typed dispatch for the six Phase A operations, thin handler adapters over existing engine logic, and the required tests.

## Important Decisions
- The task/techspec/ADR already define the design, so this run is executing that approved design rather than creating a new one.
- `KernelDeps` keeps the task-specified `AgentRegistry agent.Registry` field, and the agent package now exposes a lightweight `agent.DefaultRegistry()` handle that delegates to the existing package-level validation helpers.
- Kernel handler tests use an unexported `KernelDeps.ops` seam so `BuildDefault` can be exercised end-to-end without rewriting the existing core execution functions.
- `WorkflowSyncCommand` intentionally retains `DryRun` from legacy `core.Config` for translator parity, even though `core.SyncConfig` does not consume that field today.

## Learnings
- `internal/core/kernel/` does not exist yet in the repository.
- The task and ADR required `KernelDeps.AgentRegistry agent.Registry`, so task 04 added the missing `agent.Registry`/`agent.DefaultRegistry()` surface in `internal/core/agent`.
- `BuildDefault` now registers exactly six Phase A command handlers, and a dedicated self-test checks coverage of those registrations.
- `go test -cover ./internal/core/kernel/...` now reports `90.0%` coverage for `internal/core/kernel` and `100.0%` for `internal/core/kernel/commands`.

## Files / Surfaces
- `internal/core/api.go`
- `internal/core/plan/prepare.go`
- `internal/core/run/execution.go`
- `internal/core/fetch.go`
- `internal/core/migrate.go`
- `internal/core/sync.go`
- `internal/core/archive.go`
- `internal/core/workspace/config.go`
- `internal/core/agent/registry.go`
- `pkg/compozy/events/bus.go`
- `internal/core/kernel/` (new)
- `internal/core/kernel/commands/` (new)
- `internal/core/kernel/dispatcher_test.go`
- `internal/core/kernel/deps_test.go`
- `internal/core/kernel/commands/commands_test.go`

## Errors / Corrections
- `make verify` initially failed lint on the task-mandated `KernelDeps` name, a path-join literal in tests, and a `rangeValCopy`; the final implementation keeps a narrow documented `revive` suppression only for the spec-mandated exported type name and fixes the other two issues directly.

## Ready for Next Run
- Task 04 is implemented and verified with `go test ./internal/core/kernel/...`, `go test -race ./internal/core/kernel/...`, `go test -cover ./internal/core/kernel/...`, and full `make verify`.
- Task 08 can construct CLI kernel dependencies with `agent.DefaultRegistry()` and `kernel.BuildDefault(...)` without introducing a separate agent-registry bootstrap path.
