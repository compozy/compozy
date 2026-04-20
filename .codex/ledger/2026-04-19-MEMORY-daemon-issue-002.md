Goal (incl. success criteria):

- Resolve CodeRabbit review batch issue `002` for PR `116`, round `003`, by fixing the daemon-backed `tasks run` positional-slug flow in `internal/cli/daemon_commands.go`.
- Success means: non-interactive `compozy tasks run <slug>` no longer trips interactive form collection when the slug is explicit, regression coverage exists, `.compozy/tasks/daemon/reviews-003/issue_002.md` is triaged and closed correctly, and fresh `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the batch execution contract.
- Required skills read this session: `cy-fix-reviews`, `cy-final-verify`, `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`.
- Keep production scope to `internal/cli/daemon_commands.go`; test edits may extend to the minimum necessary CLI regression coverage.
- Do not touch unrelated dirty worktree files, other review issue files, or use destructive git commands.
- Completion requires fresh full verification via `make verify`.

Key decisions:

- Treat the review issue as valid against the tracked code at batch start: `runTaskWorkflow` invoked `maybeCollectInteractiveParams` before `resolveTaskWorkflowName(args)`, so a positional slug was ignored during the no-flags/non-TTY check.
- Fix the root cause in `internal/cli/daemon_commands.go` by resolving whether the workflow is already explicit before falling back to interactive collection, instead of weakening the generic `maybeCollectInteractiveParams` helper for every command.
- Add regression coverage at the daemon-backed command-execution layer to prove positional `tasks run <slug>` works in non-interactive mode without invoking the interactive form.

State:

- Completed after focused verification and fresh `make verify`.

Done:

- Read the required skill guides and the batch context in `.compozy/tasks/daemon/reviews-003/_meta.md`.
- Read `.compozy/tasks/daemon/reviews-003/issue_002.md` completely before editing.
- Scanned daemon-related ledgers for cross-agent awareness and noted an unrelated prior session ledger for `issue_001`.
- Inspected `internal/cli/daemon_commands.go`, `internal/cli/state.go`, and the relevant CLI tests.
- Confirmed the bug report matches the tracked code path for daemon-backed `tasks run`.
- Updated `.compozy/tasks/daemon/reviews-003/issue_002.md` to `valid` with concrete root-cause analysis before editing production code.
- Fixed `internal/cli/daemon_commands.go` so daemon-backed `tasks run` only attempts interactive collection when neither a positional slug nor `--name` was provided.
- Added `TestTasksRunCommandPositionalSlugSkipsInteractiveFormWithoutTTY` in `internal/cli/root_command_execution_test.go` to cover the non-interactive positional-slug path end to end.
- Ran focused verification successfully:
- `go test ./internal/cli -run 'TestTasksRunCommand(PositionalSlugSkipsInteractiveFormWithoutTTY|NoFlagsUsesInteractiveForm|AutoModeResolvesToStreamInNonInteractiveExecution)$' -count=1`
- Ran the full repository gate successfully:
- `make verify`
- Re-ran the focused CLI verification successfully after the final issue/ledger updates:
- `go test ./internal/cli -run 'TestTasksRunCommand(PositionalSlugSkipsInteractiveFormWithoutTTY|NoFlagsUsesInteractiveForm|AutoModeResolvesToStreamInNonInteractiveExecution)$' -count=1`
- Confirmed a later `make verify` rerun failed outside this batch because `internal/core/run/ui/remote_test.go` gained a concurrent test (`TestAttachRemoteWithRealSetupHydratesCompletedSnapshotIntoController`) that currently expects a real TTY. Left that unrelated failure untouched and documented it in the scoped issue artifact per the review-fix workflow.
- Rewrote the scoped issue artifact after in-progress drift so its final status and reasoning match the actual code diff and verification evidence.

Now:

- No technical work remains; prepare the final verified handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-daemon-issue-002.md`
- `.compozy/tasks/daemon/reviews-003/{_meta.md,issue_002.md}`
- `internal/cli/{daemon_commands.go,state.go,root_command_execution_test.go}`
- `git status --short`
- `rg`
- `sed -n`
- `go test ./internal/cli -run 'TestTasksRunCommand(PositionalSlugSkipsInteractiveFormWithoutTTY|NoFlagsUsesInteractiveForm|AutoModeResolvesToStreamInNonInteractiveExecution)$' -count=1`
- `make verify`
