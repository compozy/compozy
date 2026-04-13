# Task Memory: task_10.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Completed task 10 by inserting the 14 plan/prompt/agent hook dispatches, keeping the nil-manager path intact, adding unit/integration coverage for the new seams, and finishing with clean `make verify`.

## Important Decisions

- Treat the existing PRD/TechSpec/task specification as the approved design baseline for this execution task; no separate design loop is required before implementation.
- Keep the hook boundary payloads explicit and protocol-shaped so later SDK tasks can rely on stable fields.
- Extend `model.RuntimeManager` with generic mutable/observer dispatch methods and thread that interface through the existing run/executor/prompt/agent seams instead of adding a parallel extension-only dependency path.

## Learnings

- The current codebase already has the hook dispatcher and runtime-manager lifecycle from tasks 07-09; task 10 is mainly about threading a minimal hook-dispatch seam through plan, prompt, and agent boundaries.
- The real mock extension subprocess under `internal/core/extension/testdata/mock_extension` already records `execute_hook` and `on_event`, so it can be reused for the required end-to-end tests.
- The mock extension harness can patch plan entries, prompt text/system addenda, and base64-encoded session request prompts via `COMPOZY_MOCK_APPEND_SUFFIXES_JSON`, which is enough to cover the task-10 integration path without a second fixture binary.
- `make verify` passed after updating the executor/run tests for the widened `run.Execute(..., manager)` signature; the repo finished with 1,371 passing tests and a clean build.

## Files / Surfaces

- `internal/core/model/hooks.go`
- `internal/core/model/run_scope.go`
- `internal/core/plan/prepare.go`
- `internal/core/agent/hooks.go`
- `internal/core/prompt/common.go`
- `internal/core/agent/client.go`
- `internal/core/agent/session.go`
- `internal/core/run/internal/acpshared/session_handler.go`
- `internal/core/run/internal/acpshared/command_io.go`
- `internal/core/run/run.go`
- `internal/core/run/executor/execution.go`
- `internal/core/run/exec/exec.go`
- `internal/core/extension/hooks_integration_test.go`
- `internal/core/extension/testdata/mock_extension/main.go`
- `internal/core/plan/prepare_test.go`
- `internal/core/prompt/prompt_test.go`
- `internal/core/agent/client_test.go`
- `internal/core/run/internal/acpshared/session_handler_test.go`

## Errors / Corrections

- `make verify` initially failed because executor/run tests still called the old `run.Execute(...)` signature; those tests were updated to pass the new runtime-manager argument.
- Lint then caught nil-guard issues in the new test helpers and two now-unused agent hook payload structs; both were cleaned up before the final verify run.

## Ready for Next Run

- Task 10 is complete and verified. The remaining dirty tracking files in this workspace predate this run and should stay untouched unless the next task explicitly updates them.
