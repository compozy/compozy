Goal (incl. success criteria):

- Complete `task_04` Phase 3 package-level splits: add `internal/core/contentconv`, `internal/core/run/{exec,transcript,ui,executor}`, and `internal/core/migration`; keep `internal/core/run` and `internal/core` as thin facades; pass `make verify`.

Constraints/Assumptions:

- Follow `.compozy/tasks/refac/task_04.md`, `20260406-summary.md`, `20260406-agent-run.md`, workflow memory, `AGENTS.md`, and `CLAUDE.md`.
- Scope is structural refactoring only; minimize logic changes.
- Do not touch unrelated dirty files already present in `git status`.
- Must update task memory during execution and shared memory only for durable cross-task facts.
- Must run `make verify` before any completion claim or commit.

Key decisions:

- Use `internal/core/contentconv` as the single source for model<->kinds content/session update translation before moving UI/executor code.
- Preserve `internal/core/run` shared types in-place as the dependency anchor to avoid circular imports during the split.
- Use incremental package verification after each extraction, with `go list`/targeted tests as intermediate signals before the final `make verify`.
- Introduce a `run`-scoped shared internal package for runtime-only types/helpers before the `exec`/`executor` split; otherwise `run` facades importing subpackages would force `run -> subpackage -> run` cycles.
- Keep incomplete package probes out of the tree; only land extractions that still pass `make verify` after the move.
- Extract `run/ui` before the final `run/exec` and `run/executor` cuts because both future packages depend on the UI surface (`setupUI` / wait / quit handling) and on shared ACP/session plumbing.

State:

- Completed after clean `make verify`.

Done:

- Read repo instructions, required skill guides, workflow memory, task spec, `20260406-summary.md`, `20260406-agent-run.md`, and verification notes.
- Captured baseline package graph with `go list ./internal/core/run/... ./internal/core/...`; current graph has only `internal/core/run` and `internal/core/run/journal`.
- Confirmed `run` is still monolithic and `internal/core/migrate.go` still exists in root `core`.
- Extracted `internal/core/contentconv`, replaced the duplicated conversion switches in `run/events.go` and `run/ui_model.go`, added parity tests, and passed `make verify`.
- Extracted `internal/core/run/transcript`, moved the session view model/rendering/tool summary logic there, replaced the old `run` files with compatibility wrappers/aliases, and passed `make verify`.
- Added `internal/core/run/internal/runshared` with exported runtime-only state/helper types to support the upcoming cross-package moves without importing the parent `run` package.
- Probed `run/exec` in isolation, captured its real dependency set, then removed the incomplete copy so the repository returned to a clean verified state.
- Extracted `internal/core/run/ui`, moved the UI tests with it, added a temporary `run` bridge (`setupUI` + local `uiSession` adapter), updated preflight to call the new validation-form entry point, and passed `make verify`.
- Fixed a pre-existing race detector failure exposed by the package move by serializing the small set of tests that mutate process stdio.
- Extracted `internal/core/migration`, copied the small workflow-target resolver it needed, moved the migration tests with it, left `internal/core/migrate.go` as a thin forwarder, and passed `make verify`.
- Extracted `internal/core/run/internal/{acpshared,runtimeevents}` and completed the `run/exec` and `run/executor` moves onto that shared runtime layer.
- Reduced root `internal/core/run` to an 83-line production facade across `run.go`, `exec_facade.go`, and `preflight.go`, while preserving CLI-facing `Execute`, `ExecuteExec`, preflight, and persisted-exec helper exports.
- Completed the final repository gate successfully: `make verify` passed with clean fmt/lint, 987 tests, and build.

Now:

- Tracking and verification are complete; only optional session-ledger cleanup remains.

Next:

- If needed, create the local task commit from the verified package split.

Open questions (UNCONFIRMED if needed):

- Resolved: `run/executor` and `run/exec` both depend on `run/ui` through the narrow `runshared.UISession` interface rather than through direct controller types.
- Resolved: a small root `run` test/facade surface must remain to verify delegation and preserve CLI compatibility exports.
- Resolved: ACP session helpers belong in `internal/core/run/internal/acpshared` alongside `runshared`, not duplicated inside `exec` or `executor`.

Working set (files/ids/commands):

- `.compozy/tasks/refac/task_04.md`
- `.compozy/tasks/refac/20260406-summary.md`
- `.compozy/tasks/refac/20260406-agent-run.md`
- `.compozy/tasks/refac/memory/MEMORY.md`
- `.compozy/tasks/refac/memory/task_04.md`
- `internal/core/run/*.go`
- `internal/core/contentconv/*`
- `internal/core/run/internal/runshared/*`
- `internal/core/run/transcript/*`
- `internal/core/migrate.go`
- Baseline commands: `git status --short`, `go list ./internal/core/run/... ./internal/core/...`, `rg -n ... internal/core/run internal/core/migrate.go`
- Verification: `go test ./internal/core/contentconv ./internal/core/run`, `make verify`
- Exec probe commands: `go test ./internal/core/run/exec` to enumerate missing helpers/types before landing the move
