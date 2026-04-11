# Task Memory: task_09.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Wire executable extension activation into the real CLI entry points.
- `start` and `fix-reviews` must always request executable extensions.
- `exec` must default to disabled and only enable them via `--extensions`.
- Manager startup must happen before planning, and teardown must preserve cancellation into `RunScope.Close`.

## Important Decisions

- The per-command extension toggle will be carried as per-invocation runtime state instead of a static root-dispatcher dependency.
- Manager startup will happen in kernel handling after scope allocation so task 07/08 run-scope construction remains unchanged.
- Scoped exec execution will reuse the opened run journal/run artifacts when extensions are enabled so startup, audit, and teardown happen on the same run scope.
- Fast-path exec stays scope-free when `--extensions` is absent, so ad hoc prompts keep their previous zero-extension behavior and avoid extension audit artifacts entirely.

## Learnings

- The current `runStartHandler` only uses static dispatcher `OpenRunScopeOptions`, which cannot represent `start=true`, `fix-reviews=true`, and `exec` opt-in on the same dispatcher.
- `exec` currently bypasses `OpenRunScope` entirely and opens its own ephemeral/persisted run state, so extension-enabled exec needs dedicated scope reuse.
- `plan.Prepare` already stores the passed scope on `SolvePreparation`, but the handler still needs to start the manager and preserve cancellation into deferred teardown.
- Package-wide CLI coverage remains below 80% because the package owns a broad command surface, but the touched task files satisfy the requirement: `internal/cli/commands.go` reports 100.0% function coverage and `internal/core/kernel/handlers.go` reports 91.7% coverage on `runStartHandler.Handle`.

## Files / Surfaces

- `internal/cli/commands.go`
- `internal/cli/commands_test.go`
- `internal/cli/state.go`
- `internal/cli/root_test.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/testdata/exec_help.golden`
- `internal/core/api.go`
- `internal/core/extension/manager.go`
- `internal/core/extension/manager_spawn.go`
- `internal/core/extension/manager_shutdown.go`
- `internal/core/kernel/commands/run_start.go`
- `internal/core/kernel/commands/commands_test.go`
- `internal/core/kernel/handlers_extensions_test.go`
- `internal/core/kernel/run_scope_cancellation_integration_test.go`
- `internal/core/kernel/handlers.go`
- `internal/core/kernel/deps_test.go`
- `internal/core/model/runtime_config.go`
- `internal/core/model/run_scope.go`
- `internal/core/run/run.go`
- `internal/core/run/exec/exec.go`
- `internal/core/run/exec/exec_integration_test.go`
- `internal/core/run/run_test.go`

## Errors / Corrections

- `make verify` initially failed on `gocyclo` and `noctx`; the fix was to split `runStartHandler` into explicit fast-exec / scope-open / prepared-run helpers and switch mock-extension test builds to bounded `exec.CommandContext(...)`.
- `golangci-lint --fix` also emitted a staticcheck warning for nil-context helper calls in `internal/core/extension/runtime_test.go`; replacing those with explicit background contexts removed the warning so final verification was clean.

## Ready for Next Run

- Task implementation and verification are complete. Remaining closeout is task tracking plus the code-only local commit required by the workflow.
