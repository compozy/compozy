# Task Memory: task_08.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Create the reusable QA planning artifact set for daemon improvements under `.compozy/tasks/daemon-improvs/analysis/qa/` so `task_09` can execute without redefining scope, case IDs, or output paths.
- Cover canonical transport parity, timeout-class/client behavior, runtime shutdown and log policy, ACP liveness/fault handling, observability surfaces, and at least one external-workspace operator flow.

## Important Decisions

- Use `qa-output-path=.compozy/tasks/daemon-improvs/analysis` exactly as required by the task, while sourcing requirements from the current worktree files under `.compozy/tasks/daemon-improvs/`.
- Mirror the prior daemon QA artifact style, but classify flows strictly from the current repo reality: `make verify` is the broad gate, browser validation is blocked/out of scope, and many daemon integration-style tests still live in the default `go test` lane.
- Call out E2E follow-up explicitly where public daemon behavior is only protected today by integration-style tests, especially live-daemon HTTP/UDS parity and daemon-backed ACP fault surfacing.

## Learnings

- The worktree has no `web/` directory and no browser harness configuration, so browser validation cannot be planned as an executable lane for `task_09`.
- `internal/api/httpapi/transport_integration_test.go`, `internal/daemon/boot_integration_test.go`, `internal/cli/operator_commands_integration_test.go`, `internal/core/run/executor/execution_acp_integration_test.go`, and `pkg/compozy/runs/integration_test.go` are the main existing automation seams relevant to this task.
- The shared workflow memory contained stale `mage verify` / `mage testE2ERuntime` assumptions; repository files confirm the active gate is `make verify`.
- `make verify` exposed a real `internal/daemon` race unrelated to the documentation scope: watcher refresh after directory rename could lag persisted sync, so a fast follow-up write inside the renamed directory could miss filesystem coverage until `flushPendingChanges` reconciled watches before syncing.

## Files / Surfaces

- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/`
- `.compozy/tasks/daemon-improvs/analysis/qa/issues/`
- `.compozy/tasks/daemon-improvs/analysis/qa/screenshots/`
- `.compozy/tasks/daemon-improvs/{_techspec.md,_tasks.md,task_03.md,task_04.md,task_05.md,task_06.md,task_07.md,task_08.md,task_09.md}`

## Errors / Corrections

- Corrected the stale shared-memory assumption that an explicit `mage`-based integration lane exists in this worktree. Planning must use `make verify` plus focused `go test` commands that match the current repository.
- Corrected the repository verification blocker in `internal/daemon/watchers.go` by reconciling the watch set before sync when pending changes require a refresh. This restored deterministic rename-followed-by-write behavior and allowed `make verify` to pass cleanly.

## Ready for Next Run

- Task artifacts are complete and verified. If follow-up work is needed, consume `.compozy/tasks/daemon-improvs/analysis/qa/` directly from `task_09`, keeping browser validation blocked/out of scope unless a real daemon web surface lands later.
