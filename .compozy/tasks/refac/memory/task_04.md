# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Deliver the Phase 3 package split by extracting `contentconv`, `run/exec`, `run/transcript`, `run/ui`, `run/executor`, and `migration`, while leaving `run` and `core` as thin facades and finishing with clean `make verify`.
- Keep the refactor structural: move code with minimal logic changes and preserve the Task 03 parsing/provider boundaries.

## Important Decisions
- Sequence the work per task order: `contentconv` first, then `run/exec`, `run/transcript`, `run/ui`, `run/executor`, and finally `migration`.
- Keep shared runtime types in `internal/core/run` as the stable dependency anchor for sub-packages to reduce cycle risk during extraction.
- Use incremental `go list` and package-scoped test/build checks as pre-final signals between major moves.
- Add a `run`-scoped shared internal package before the `exec`/`executor` split so the `run` facades can import subpackages without creating `run -> subpackage -> run` cycles.
- Do not leave incomplete split packages checked in; use isolated compile probes to discover dependencies, then either complete the extraction or remove the probe before rerunning `make verify`.
- Extract `run/ui` before the final `run/exec` / `run/executor` moves because both future packages depend on the UI session surface and because that surface is smaller to bridge cleanly from the old `run` package.
- Keep root `internal/core/run` as a compatibility facade only: `Execute`, `ExecuteExec`, preflight wrappers, and persisted-exec helpers stay there while the implementations live in subpackages.

## Learnings
- The current package graph is still unsplit: baseline `go list ./internal/core/run/... ./internal/core/...` returns only `internal/core/run` plus `internal/core/run/journal` under `run`.
- `internal/core/run` currently contains the expected split candidates from Phase 1: dedicated files for lifecycle, shutdown, runner, session execution, session handler, render blocks, and UI panels, which should make package moves mostly mechanical.
- Shared workflow memory confirms Task 03 boundaries that must survive this refactor: parsing remains in `tasks`/`reviews`, and provider registry composition remains in `providerdefaults`.
- `internal/core/contentconv` now owns the bidirectional `model` <-> `kinds` content/session conversion, with `run/events.go` and `run/ui_model.go` reduced to direct package calls.
- The first extraction is fully verified: `go test ./internal/core/contentconv ./internal/core/run` passed, followed by a clean `make verify`.
- `internal/core/run/transcript` now owns the session transcript model, rendered block helpers, and tool-use summary logic; the old `run` files are compatibility shims only.
- The transcript extraction is also fully verified with a clean `make verify`.
- `internal/core/run/internal/runshared` now exists as the first shared dependency layer for exported runtime-only types/helpers needed by cross-package code.
- The first `exec` probe showed the concrete blockers for a real extraction: ACP session setup/logging helpers, UI setup/wait wiring, and many shared runtime field/method references all need either exported shared owners or package-local adapters before `run.ExecuteExec()` can delegate safely.
- The current dependency graph shows `exec` and `executor` both converging on the same two shared surfaces: UI session control and ACP session setup/update handling. Cutting those shared surfaces first avoids repeating the same bridge work in both later packages.
- `internal/core/run/ui` now owns the TUI, event adapter, and validation-form code. The old `run` package reaches it only through a narrow `setupUI` bridge and a preflight wrapper, which keeps the remaining `exec` / `executor` work focused on ACP/session plumbing instead of Bubble Tea internals.
- Moving the UI tests out of `run` exposed an existing package-level race in tests that swap `os.Stdout` / `os.Stderr`. The fix was to serialize the small set of stdio-mutating tests rather than weakening assertions or suppressing `-race`.
- `internal/core/migration` now owns the V1-to-V2 workflow artifact migration logic and tests. The old `internal/core/migrate.go` is reduced to a forwarding shim, which removes another large implementation file from the root `core` package without touching runtime behavior.
- The final split required an internal shared-runtime layer: `run/executor` and `run/exec` now depend on `run/internal/{runshared,acpshared,runtimeevents}` instead of on root `run`, which keeps the package graph acyclic while preserving a thin root facade.
- Root `internal/core/run` now has 83 lines of production code (`run.go`, `exec_facade.go`, `preflight.go`), satisfying the "<200 lines" success criterion while still preserving CLI compatibility.
- Final verification passed with `make verify` green: `fmt`, `lint`, `987` tests, and `build`.

## Files / Surfaces
- `internal/core/run/`
- `internal/core/contentconv/`
- `internal/core/run/exec/`
- `internal/core/run/executor/`
- `internal/core/run/internal/runshared/`
- `internal/core/run/internal/acpshared/`
- `internal/core/run/internal/runtimeevents/`
- `internal/core/run/preflight/`
- `internal/core/run/transcript/`
- `internal/core/run/ui/`
- `internal/core/migration/`
- `internal/core/migrate.go`
- `internal/core/api.go`
- `internal/core/kernel/handlers.go`
- `.compozy/tasks/refac/task_04.md`
- `.compozy/tasks/refac/_tasks.md`

## Errors / Corrections
- Initial attempt to scan `.codex/ledger/*.md` failed because the directory did not exist yet; created `.codex/ledger/` and switched to `null_glob` for safe scanning.
- Removing the old conversion switch code also removed a package-local `copyJSON` helper still used by `session_view_model.go`; restored that responsibility locally as `cloneRawJSON` in the transcript/view-model code path.
- Temporary wrapper helpers around `contentconv` triggered lint dead-code failures; removed the dead wrappers and updated remaining call sites/tests to use `contentconv` directly.
- Transcript extraction exposed hidden package-level helpers (`cloneContentBlocks`, `splitRenderedText`, and the view-model snapshot accessor); each was moved into `run/transcript` and the old `run` package now only forwards to the new owner.
- An initial `run/exec` copy compiled far enough to expose its dependency set but was not viable to keep in-tree without either moving ACP session/UI helpers too or introducing temporary nonfunctional stubs; removed the probe and kept only the new `runshared` package plus the recorded findings.

## Ready for Next Run
- Task 04 is complete and verified. Task 05 can now target DRY/generics cleanup against the stable package boundaries `contentconv`, `run/{exec,executor,transcript,ui,preflight}`, `run/internal/{runshared,acpshared,runtimeevents}`, and `migration` without reopening the package graph.
