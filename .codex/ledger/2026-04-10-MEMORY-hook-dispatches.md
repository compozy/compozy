Goal (incl. success criteria):

- Complete extensibility task 10 by inserting the 14 plan/prompt/agent hook dispatches required by `_protocol.md` section 6.5, adding the required unit/integration tests, updating workflow memory/task tracking, and finishing with clean `make verify` plus one local commit.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, `.compozy/tasks/extensibility/task_10.md`, `_techspec.md`, `_protocol.md`, `_tasks.md`, and ADRs under `.compozy/tasks/extensibility/adrs/`.
- Required skills in use: `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`; `systematic-debugging` and `no-workarounds` are active guardrails for this high-complexity runtime change.
- Brainstorming design gate is treated as already satisfied by the approved PRD/TechSpec/task workflow for this implementation task; no fresh user design loop is needed.
- Workspace is already dirty in unrelated extensibility tracking files and prior ledgers; do not touch unrelated changes or use destructive git commands.
- Final completion still requires fresh `make verify` output, plus the explicit task tests and tracking/memory updates.

Key decisions:

- Keep the hook seam nil-safe by checking for a runtime manager before constructing payloads, so the disabled-extensions path remains a no-op with no behavior change.
- Prefer passing typed payload structs through the dispatcher rather than loose maps where possible, so protocol fields stay explicit and patch application can decode back into the original shapes.
- Reuse the existing spawned mock-extension harness for end-to-end coverage instead of inventing a second fake runtime path.

State:

- Completed after implementation, task-specific tests, and clean `make verify`.

Done:

- Read repository instructions, required skill files, workflow memory, task 10 spec, `_techspec.md`, `_protocol.md`, `_tasks.md`, and ADRs.
- Scanned existing ledgers for cross-agent awareness, especially prior extensibility tasks 07-09 and the task-08 lifecycle details.
- Confirmed the workspace is already dirty before edits and identified the unrelated files that must remain untouched.
- Captured the pre-change signal: `internal/core/plan/prepare.go`, `internal/core/prompt/common.go`, `internal/core/agent/client.go`, and `internal/core/agent/session.go` currently contain no task-10 hook dispatches, and current prompt/session inputs do not yet carry all hook context required by protocol section 6.5.
- Inspected the existing hook dispatcher, runtime scope seam, ACP session handling, and the mock extension harness to plan the integration path.
- Widened `model.RuntimeManager` with generic mutable/observer hook-dispatch methods and threaded that interface through `run.Execute`, `runshared.Config`, scoped exec, prompt batch params, ACP session requests, and the ACP session-update handler.
- Inserted all required task-10 hook dispatches across plan (`prepare.go`), prompt (`common.go`), and agent/ACP seams (`client.go`, `command_io.go`, `session_handler.go`) while preserving the nil-manager behavior.
- Added protocol-shaped tests in `prepare_test.go`, `prompt_test.go`, `client_test.go`, and `session_handler_test.go`, plus a real subprocess integration test in `internal/core/extension/hooks_integration_test.go`.
- Extended the spawned mock extension harness with `COMPOZY_MOCK_APPEND_SUFFIXES_JSON` to mutate plan entries, prompt text/addenda, and base64-encoded agent session prompts in integration tests.
- Updated executor/run tests for the widened `run.Execute(..., manager)` signature.
- Passed the required verification gate: `make verify` succeeded with 1,371 tests and a clean build.

Now:

- Review the final diff and create the local task commit.

Next:

- Optional cleanup only: remove this ledger after the commit if no further follow-up is needed in this session.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.compozy/tasks/extensibility/task_10.md`
- `.compozy/tasks/extensibility/_techspec.md`
- `.compozy/tasks/extensibility/_protocol.md`
- `.compozy/tasks/extensibility/memory/MEMORY.md`
- `.compozy/tasks/extensibility/memory/task_10.md`
- `internal/core/model/run_scope.go`
- `internal/core/model/hooks.go`
- `internal/core/plan/prepare.go`
- `internal/core/prompt/common.go`
- `internal/core/agent/hooks.go`
- `internal/core/agent/client.go`
- `internal/core/agent/session.go`
- `internal/core/run/internal/acpshared/command_io.go`
- `internal/core/run/internal/acpshared/session_handler.go`
- `internal/core/extension/{dispatcher.go,manager.go,runtime.go,hooks_integration_test.go,testdata/mock_extension/main.go}`
