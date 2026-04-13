# Task Memory: task_07.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Move run artifact/journal/bus allocation ahead of `plan.Prepare`, add the optional extension-aware manager bootstrap, and keep disabled-extension behavior unchanged for current commands.

## Important Decisions
- Added a neutral `model.RunScope` interface plus `model.OpenRunScope` factory so `kernel`, `plan`, and legacy `core` paths can depend on early run-scope bootstrap without importing `internal/core/extension` directly.
- Kept the concrete extension-aware implementation in `internal/core/extension/runtime.go`; the extension package registers itself as the active run-scope factory while the model package retains a basic fallback scope for non-extension contexts and isolated tests.
- `SolvePreparation` now owns a run scope instead of a bare journal handle so execution can recover the pre-opened event bus and close the whole scope after runs.

## Learnings
- The existing `internal/core/extension` package already imports `internal/core/kernel` via Host API helpers, so moving `plan`/`kernel` onto a concrete extension type would create a package cycle. The model-level factory seam avoids that without splitting the extension package.
- `run.Execute` already closes the journal, so the post-run scope close path must be tolerant of an already-closed journal and still close the run-scoped event bus.
- The direct `internal/core/api.go` adapters also need to allocate their scope through `model.OpenRunScope`; otherwise the repository would retain two competing run-resource allocation paths and task 07's single-entry invariant would be false.

## Files / Surfaces
- `internal/core/model/run_scope.go`
- `internal/core/model/preparation.go`
- `internal/core/extension/runtime.go`
- `internal/core/plan/prepare.go`
- `internal/core/kernel/handlers.go`
- `internal/core/kernel/run_scope_integration_test.go`
- `internal/core/extension/runtime_test.go`
- `internal/core/api.go`
- `internal/cli/root.go`
- `compozy.go`

## Errors / Corrections
- Fixed a typed-nil interface bug in `RunScope.RunManager()` after the new accessor tests showed it was returning a non-nil `model.RuntimeManager` when the concrete manager pointer was nil.
- Raised extension-package coverage above the task threshold by adding direct runtime bootstrap/shutdown tests instead of weakening existing assertions.

## Ready for Next Run
- Task 07 implementation is complete and `make verify` passed cleanly after the refactor and test updates.
- Task 08 can assume early run-scope bootstrap is available through `model.OpenRunScope`, that `SolvePreparation` carries the scope, and that the pre-start manager handle is exposed via `prep.RuntimeManager()`.
