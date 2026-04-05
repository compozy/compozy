# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Add `compozy start` task-validation preflight with Bubble Tea TTY recovery, non-TTY `stderr` fix prompt behavior, `--skip-validation` / `--force`, structured decision logging, and required tests.

## Important Decisions

- Reuse the existing task-registry resolution helper used by `validate-tasks` instead of adding another workspace-config code path.
- Treat the PRD/TechSpec/ADR as the approved design source; no separate brainstorming/design-doc loop for this implementation task.
- Keep the task-requested `PreflightCheck` wrapper, but add `PreflightCheckConfig` so the CLI can inject `--skip-validation`, structured logger output, and the command stderr writer without depending on process globals.
- Keep bundled-skill preflight first in `runPrepared`; task validation runs immediately afterward so the existing skill-refresh behavior stays stable.

## Learnings

- `internal/cli/root.go` currently runs bundled-skill preflight and then immediately calls `core.Run`; there is no task-metadata preflight seam yet.
- `newStartCommand()` currently exposes only `--name`, `--tasks-dir`, and `--include-completed` beyond the common runtime flags.
- `internal/cli/validate_tasks.go` already has the resolved task registry helper needed by start preflight.
- The Bubble Tea v2 model interface in this repo returns `tea.View`, not `string`, and `cmd.ErrOrStderr()` is the right sink for both slog preflight logs and non-TTY fix prompts in tests.

## Files / Surfaces

- `internal/cli/root.go`
- `internal/cli/root_test.go`
- `internal/cli/validate_tasks.go`
- `internal/cli/validate_tasks_test.go`
- `internal/core/run/preflight.go`
- `internal/core/run/preflight_test.go`
- `internal/core/run/validation_form.go`
- `internal/core/run/validation_form_test.go`
- `internal/core/tasks/validate.go`
- `internal/core/tasks/fix_prompt.go`

## Errors / Corrections

- Task docs conflict on `PreflightCheck`: one section lists a signature without `skipValidation`, but the required unit tests expect a skipped-preflight path inside the preflight API. Resolved by keeping the requested wrapper and adding `PreflightCheckConfig` for the skip path and testable I/O injection.

## Ready for Next Run

- Verification completed: `make verify` passed after the preflight/runtime/test changes.
- Remaining closeout is tracking-file status updates plus the single local commit for this task.
