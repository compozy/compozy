# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement the daemon-owned sequential `task_multi` parent coordinator behind the task_02 multi-run API stubs. The coordinator must preflight all slugs before parent creation, start normal `task` child runs one at a time with `ParentRunID`, emit reconstructable parent queue events, and cancel active/queued work from parent cancellation.
- Completed the coordinator, snapshot reconstruction, parent/child linkage, cancellation behavior, and run manager integration coverage for task_03.

## Important Decisions
- Treat `parallel` requests as accepted V1 input but execute with enqueued semantics in the daemon, matching the shared workflow memory and PRD/ADR contract.
- Use parent runtime override `run_id` only for the parent run and strip it from child runtime overrides so a single multi-run request does not force duplicate child run IDs.

## Learnings
- Baseline focused daemon test fails at compile time because `task_multi` mode, multi-run snapshot reconstruction, item status constants, and queue event kinds are not implemented yet.
- `make verify` initially failed on `gocyclo` in `runTaskMultiCoordinator`; extracting one-child execution into `runTaskMultiChildAt` fixed the lint failure without changing behavior.
- Parent cancellation events must be written with a detached context so queued/active item cancellation is still reconstructable after the parent context is canceled.

## Files / Surfaces
- `internal/daemon/task_multi_test.go`
- `internal/daemon/task_multi.go`
- `internal/daemon/run_manager.go`
- `internal/daemon/service.go`
- `internal/daemon/task_transport_service.go`
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/kinds/task.go`
- `pkg/compozy/events/kinds/payload_compat_test.go`

## Errors / Corrections
- Corrected the initial coordinator shape to keep cyclomatic complexity under the repository lint threshold.

## Ready for Next Run
- `make verify` passed after implementation. task_04 can rely on `StartRunMultiple` creating a durable `task_multi` parent and normal child `task` rows with `ParentRunID`; task_05 can reconstruct queue state from the parent snapshot/events.

---

## V2 — Add Parallel CLI Controls and Request Wiring (worktree-backed parallel)

### Objective Snapshot (V2)
- Add `--parallel` and `--parallel-limit` to `tasks run`, resolve mode/limit precedence, reject invalid combos before daemon contact, forward resolved mode + limit through the task_02 `StartTaskRunMultiple` contract. CLI-only; daemon parallel execution is task_06.

### Important Decisions (V2)
- CLI mode resolution now honors `parallel` (no downgrade): precedence `--parallel` > `cfg.Tasks.Run.EffectiveRunMultipleMode()` > `enqueued`. The V1 "fallback-to-enqueued" stderr message was removed entirely.
- `--parallel` uses `commandFlagChanged` + the bool value, so `--parallel=false` forces enqueued (explicit override either direction).
- Limit resolution precedence `--parallel-limit` > `cfg.Tasks.Run.EffectiveRunMultipleParallelLimit()` > `workspace.DefaultRunMultipleParallelLimit` (2). Resolved limit is validated `> 0` before daemon contact.
- `ParallelLimit` is only set on the request when the resolved mode is `parallel` (matches "effective only when parallel"); enqueued requests omit it.
- `--parallel`/`--parallel-limit` are rejected (exit 1) on the single-run path via `rejectMultipleOnlyParallelFlags(cmd)`, placed right after the `--multiple` branch check in `runTaskWorkflow`.

### Learnings (V2)
- The daemon still rejects `parallel` in `internal/daemon/task_multi.go:resolveTaskMultiMode` (`unsupported_run_multiple_mode`, 422) until task_06. So forwarding parallel through the REAL in-process daemon now fails; the prior in-process "parallel config streams enqueued" test was rewritten to assert the daemon rejection (`parallel run_multiple mode is not supported by the daemon`).
- In-process test client returns the daemon `*Problem` directly (not an `apiclient.RemoteError`), so `mapDaemonCommandError` maps it to exit code 2; over HTTP a 422 maps to exit 1. Don't pin the in-process rejection test to a specific exit code — assert it's a non-zero `commandExitError`.
- Golden help (`internal/cli/testdata/tasks_run_help.golden`) is an exact-match test with no `-update` flag; regenerate via `env -u GOROOT go run ./cmd/compozy tasks run --help > <golden>`.
- Help-text lint trap: `tasks run` help forbids the substring `--concurrent`; phrase `--parallel-limit` help without it.

### Files / Surfaces (V2)
- `internal/cli/state.go` (added `parallel bool`, `parallelLimit int` to `runtimeConfig`)
- `internal/cli/daemon_commands.go` (flags, `rejectMultipleOnlyParallelFlags`, rewritten `resolveTaskRunMultipleMode`, new `resolveTaskRunMultipleParallelLimit`, request wiring, help Long/Example)
- `internal/cli/testdata/tasks_run_help.golden`
- `internal/cli/daemon_commands_test.go`, `internal/cli/root_command_execution_test.go`, `internal/cli/root_test.go`

### Ready for Next Run (V2)
- task_06 (scheduler refactor) must make `resolveTaskMultiMode` accept `parallel` and consume `body.ParallelLimit`; once it does, the in-process rejection test (`TestTasksRunMultipleCommandInProcessParallelModeRejectedByDaemon`) should be flipped to assert a successful parallel run.
