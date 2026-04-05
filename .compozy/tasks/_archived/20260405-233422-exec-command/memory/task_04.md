# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Align active docs, command help snapshots, and regression tests with the shipped `compozy exec` flow and `.compozy/runs/<run-id>/` artifact model.
- Close task 04 without reopening runtime behavior from tasks 01-03.

## Important Decisions
- README drift counts as part of the shipped CLI contract for this task, so docs updates will cover runtime flag defaults/names that no longer match current help output.
- `exec` help text should mention `.compozy/runs/<run-id>/` directly so the golden fixture locks artifact-location expectations, not just flags.
- Regression coverage should read active docs/help fixtures and fail if `.tmp/codex-prompts` reappears on active product surfaces.
- Keep task tracking and workflow memory updates out of the automatic code commit; update them in the workspace, but stage only product/docs/test files for the required local commit.

## Learnings
- Active non-task product surfaces no longer reference `.tmp/codex-prompts`; the remaining gap is missing or stale documentation rather than leftover runtime strings.
- README is currently the main active user-facing doc surface and is missing:
- a dedicated `compozy exec` section
- `.compozy/runs/` artifact documentation
- `[exec]` workspace config coverage
- updated runtime flag values/names (`cursor-agent`, `copilot`, `tail-lines = 0`)
- Temp-directory paths surfaced a macOS-specific assertion issue: exec JSON payloads may return canonicalized `/private/var/...` paths even when tests create workspaces under `/var/...`, so artifact-path assertions should compare resolved paths instead of raw strings.

## Files / Surfaces
- `README.md`
- `internal/cli/root.go`
- `internal/cli/root_test.go`
- `internal/cli/testdata/exec_help.golden`
- `internal/cli/root_command_execution_test.go`
- `internal/core/run/execution_acp_integration_test.go`
- `.compozy/tasks/exec-command/task_04.md`
- `.compozy/tasks/exec-command/_tasks.md`

## Errors / Corrections
- `sed` attempt for `.compozy/tasks/exec-command/references/tracking-checklist.md` failed because that path does not exist; if a tracking checklist is still needed later, locate the actual file before using it.
- A quick shell check used `status` in zsh, which is read-only; use a different variable name for exit-code capture in later commands.
- The first JSON CLI contract test compared raw temp paths and failed on `/var` vs `/private/var`; fixed by resolving symlinks in the test helper instead of changing runtime output.

## Ready for Next Run
- Task complete after:
- `go test ./internal/cli -count=1`
- `go test ./internal/core/run -count=1`
- `./bin/compozy validate-tasks --name exec-command`
- `make verify`
- Product changes committed as `4c33280` (`docs: align exec docs and regression coverage`).
