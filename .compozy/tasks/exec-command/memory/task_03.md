# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement `compozy exec` as a first-class shared-runtime mode with prompt-source validation, prompt-backed preparation, text/json output handling, and persisted machine-readable results under `.compozy/runs/<run-id>/`.

## Important Decisions
- Reuse the already-approved task docs, techspec, and ADRs as the design contract for this execution task; do not create a parallel implementation path.
- Keep prompt-source resolution in CLI/state plumbing so runtime validation receives exactly one resolved source.
- Keep executor changes shared and mode-aware so JSON output suppresses UI/presentation without bypassing ACP session execution.
- Materialize stdin and prompt-file contents into `ResolvedPromptText` while preserving the original raw prompt-source fields for validation and metadata.
- Create prompt and empty log artifacts during preparation so dry-run exec still leaves a complete `.compozy/runs/<run-id>/jobs/` artifact set.

## Learnings
- Task 01 and task 02 already added the config surface (`OutputFormat`, `PromptText`, `PromptFile`, `ReadPromptStdin`) and run-artifact layout (`RunArtifacts`), so task 03 is mostly about wiring and result contracts rather than adding new foundations.
- Current planner input resolution still only supports PRD-task and PR-review directory modes, which is the main pre-change gap for prompt-backed exec.
- The root command tree already has `commandKindExec` and workspace-config hooks for `[exec]`, but the actual Cobra command is not registered yet.
- JSON mode needed two separate suppressions: disabling Bubble Tea UI and preventing non-UI ACP log mirroring to process stdout/stderr while still writing job log files.
- Existing `withWorkingDir` CLI execution tests needed the same cwd mutex used elsewhere in the package; without it, new exec command tests could observe unrelated workspace state.

## Files / Surfaces
- `.codex/CONTINUITY-exec-command.md`
- `internal/cli/root.go`
- `internal/cli/root_test.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/testdata/exec_help.golden`
- `internal/cli/workspace_config.go`
- `internal/core/api.go`
- `internal/core/plan/input.go`
- `internal/core/plan/prepare.go`
- `internal/core/plan/prepare_test.go`
- `internal/core/run/types.go`
- `internal/core/run/execution.go`
- `internal/core/run/command_io.go`
- `internal/core/run/execution_acp_test.go`
- `internal/core/run/execution_acp_integration_test.go`
- `internal/core/run/result.go`
- `internal/core/run/result_test.go`
- `test/public_api_test.go`

## Errors / Corrections
- Baseline verification before edits showed the feature is not wired yet: no `newExecCommand()` exists, the root command does not register `exec`, and `resolveInputs` only branches between PRD-task and PR-review modes.
- Focused tests initially leaked temporary run artifacts into `internal/core/plan/.compozy/` because one review-preparation test omitted `WorkspaceRoot`; fixed the test and removed the generated scratch directory.
- Final cleanup found the same leak pattern in `test/public_api_test.go`; setting `WorkspaceRoot` there keeps public API tests from writing shared run artifacts into `test/.compozy/`.
- JSON-mode execution tests initially showed ACP session traffic on stderr because non-UI logging still mirrored streams to the process; fixed `createLogWriters` to separate “no UI” from “human output enabled”.

## Ready for Next Run
- Task 03 is implemented, verified, and committed locally as `6eb2201` (`feat: add shared exec command pipeline`). Final verification after the public API test cleanup passed via `go test ./test` and `make verify`.
