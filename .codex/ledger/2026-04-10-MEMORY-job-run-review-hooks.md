Goal (incl. success criteria):

- Complete extensibility task 11 by inserting the remaining job/run/review/artifact hook dispatches, adding unit and spawned-extension integration tests, updating task memory/tracking, and finishing with clean `make verify` plus one local commit if verification passes.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/extensibility/task_11.md`, `_techspec.md`, `_protocol.md`, `_tasks.md`, and ADR-004.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`; `systematic-debugging` and `no-workarounds` are active guardrails for this high-complexity runtime change.
- Brainstorming is treated as already satisfied by the approved PRD/TechSpec/task workflow for this implementation task; no new design loop is needed.
- Workspace is already dirty in unrelated extensibility tracking files and prior ledgers; do not touch unrelated changes or use destructive git commands.
- Final completion still requires targeted task tests, fresh `make verify`, memory/tracking updates, self-review, and careful staging that excludes unrelated files.

Key decisions:

- Reuse the existing task-10 generic hook seam (`model.DispatchMutableHook` / `DispatchObserverHook`) instead of adding extension-specific plumbing in executor/review/host-write paths.
- Reuse the spawned mock-extension harness for integration coverage, extending its patch behavior only where protocol-shaped task-11 payloads require it.
- Formalize task-11-only hook payload structs close to the insertion sites so the protocol surface stays explicit and nil-manager semantics remain a cheap early return.

State:

- Completed after implementation, task-specific coverage checks, and a clean `make verify`.

Done:

- Read repository instructions, required skill files, workflow memory, task 11 spec, `_techspec.md`, `_protocol.md`, `_tasks.md`, and ADR-004.
- Scanned related ledgers for task 08, 09, and 10 to preserve bootstrap/lifecycle/hook context.
- Confirmed pre-change signal: task-11 hook names appear only in `internal/core/extension/manifest.go`; there are no task-11 dispatch call sites yet.
- Confirmed there are currently no concrete `FetchConfig`, `FixOutcome`, or `RunSummary` types in the repo, so task-11 needs to decide and codify the runtime payload shapes for those protocol rows.
- Identified the likely insertion seams: `internal/core/run/executor/{execution.go,runner.go,shutdown.go,review_hooks.go}` and `internal/core/extension/host_writes.go`.
- Added concrete hook payload helpers in `internal/core/model/hook_types.go` plus executor-local payload helpers in `internal/core/run/executor/hooks.go`.
- Inserted job/run hooks in the executor flow, including retry veto handling and run shutdown observer boundaries.
- Inserted review hooks across planning and post-fix resolution, with `review.pre_resolve` able to suppress remote provider resolution via `resolve=false`.
- Inserted artifact hooks in the shared `writeArtifactFile` path used by both `host.artifacts.write` and `host.tasks.create`, including explicit `cancelled_by_extension` errors.
- Added unit coverage in `prepare_test.go`, `execution_test.go`, and `host_writes_test.go`, plus spawned-extension integrations in `hooks_integration_test.go` for run/job, review, and artifact phases.
- Fixed a batching regression where hook-renamed review groups were losing their new code-file keys before job preparation.
- Refactored the touched planner/executor functions to satisfy `funlen` / `gocyclo` lint rules without changing behavior.
- Passed targeted coverage checks (`internal/core/run/executor 80.6%`, `internal/core/extension 80.4%`) and a clean `make verify` (`1384` tests, zero lint issues, successful build).

Now:

- Update task tracking and create the local commit for task 11.

Next:

- Optional cleanup only: remove this ledger after the local commit if no follow-up is needed in this session.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-10-MEMORY-job-run-review-hooks.md`
- `.compozy/tasks/extensibility/task_11.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/_protocol.md`
- `.compozy/tasks/extensibility/adrs/adr-004.md`
- `.compozy/tasks/extensibility/memory/MEMORY.md`
- `.compozy/tasks/extensibility/memory/task_11.md`
- `internal/core/run/executor/execution.go`
- `internal/core/run/executor/runner.go`
- `internal/core/run/executor/hooks.go`
- `internal/core/run/executor/review_hooks.go`
- `internal/core/model/hook_types.go`
- `internal/core/plan/prepare.go`
- `internal/core/extension/host_writes.go`
- `internal/core/extension/host_api_errors.go`
- `internal/core/extension/runtime.go`
- `internal/core/extension/hooks_integration_test.go`
- `internal/core/run/executor/execution_test.go`
- Commands: `go test ./internal/core/{plan,extension}`, `go test ./internal/core/run/executor`, `go test -cover`, `make verify`
