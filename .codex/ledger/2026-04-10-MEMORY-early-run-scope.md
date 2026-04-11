Goal (incl. success criteria):

- Complete extensibility task 07 by introducing early run-scope bootstrap before `plan.Prepare`, including `OpenRunScope`, `RunScope`, optional extension-manager construction, ordered teardown, and tests.
- Success requires the documented task deliverables, updated workflow memory/tracking, clean targeted tests, clean `make verify`, and one local commit.

Constraints/Assumptions:

- Follow repository `AGENTS.md`/`CLAUDE.md`, task 07, `_techspec.md`, `_tasks.md`, and ADR-002.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`; `systematic-debugging` and `no-workarounds` are active guardrails for the high-complexity refactor.
- Keep scope tight to early bootstrap. Do not add subprocess startup, hook insertion points, or CLI-visible extension toggles in this task.
- Do not touch unrelated dirty worktree entries outside the task’s required memory/tracking updates.

Key decisions:

- Treat `OpenRunScope` as the single allocation path for run artifacts, journal, bus, and optional manager so `plan.Prepare` stops creating those resources itself.
- Thread the run scope through preparation via shared runtime-handle plumbing instead of duplicating bus/journal ownership across packages.
- Keep executable extensions opt-in through `OpenRunScopeOptions`; default command behavior remains disabled in this task.

State:

- Complete.

Done:

- Read repository instructions, required skill files, shared workflow memory, current task memory, task 07 spec, `_techspec.md`, `_tasks.md`, and ADR-002.
- Scanned existing session ledgers for extension-related prior work.
- Confirmed no blocking contradictions across task 07, the TechSpec, and ADR-002.
- Inspected current wiring in `internal/core/kernel/handlers.go`, `internal/core/plan/prepare.go`, `internal/core/model/preparation.go`, `internal/core/api.go`, `internal/core/run/executor/execution.go`, and `internal/core/extension/*`.
- Captured the pre-change signal: `plan.Prepare` still allocates run artifacts/journal itself and the extension package has no run-scope bootstrap or manager type yet.
- Added the neutral `model.RunScope` / `model.OpenRunScope` seam and moved early run-resource allocation behind that single bootstrap entry point.
- Implemented `internal/core/extension/runtime.go` with optional executable-extension manager bootstrap and ordered teardown.
- Threaded run scopes through `kernel`, `plan`, `model.SolvePreparation`, and direct `core` adapters so planning executes against pre-opened runtime resources.
- Added runtime/unit/integration coverage for disabled/enabled manager modes, teardown ordering, nil-manager behavior, and run-start integration.
- Ran targeted package verification:
  - `go test ./internal/core/model ./internal/core/plan ./internal/core/kernel ./internal/core/extension ./internal/core/run/executor`
  - `go test ./internal/core/extension -cover` (`80.0%`)
- Ran full repository verification with `make verify` and it passed cleanly.
- Updated workflow memory plus task 07 tracking (`task_07.md` and `_tasks.md`) after verification completed.
- Created local commit `76d29e8` (`refactor: bootstrap run scope before planning`).
- Re-ran `make verify` on the committed tree and it passed cleanly.

Now:

- None.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-early-run-scope.md`
- `.compozy/tasks/extensibility/task_07.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/_tasks.md`
- `.compozy/tasks/extensibility/adrs/adr-002.md`
- `.compozy/tasks/extensibility/memory/MEMORY.md`
- `.compozy/tasks/extensibility/memory/task_07.md`
- `.compozy/tasks/extensibility/_tasks.md`
- `.compozy/tasks/extensibility/task_07.md`
- `internal/core/extension/*.go`
- `internal/core/kernel/handlers.go`
- `internal/core/model/preparation.go`
- `internal/core/plan/prepare.go`
- `internal/core/api.go`
- `internal/core/run/executor/execution.go`
- Commands: `rg`, `sed`, `git status --short`, targeted `go test`, `go test ./internal/core/extension -cover`, `make verify`
