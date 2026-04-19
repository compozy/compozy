Goal (incl. success criteria):

- Resolve the scoped CodeRabbit review batch item `.compozy/tasks/daemon/reviews-003/issue_003.md` for PR `116`, round `003`.
- Success means: the stale `validate-tasks` assertion messages in `internal/cli/migrate_command_test.go` are updated to the current `tasks validate` command name, the issue file is triaged and closed correctly, and fresh `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the batch execution contract.
- Required skills read this session: `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`.
- Only `.compozy/tasks/daemon/reviews-003/issue_003.md` is in scope for review-artifact edits.
- Code-file scope is limited to `internal/cli/migrate_command_test.go`.
- The worktree already contains unrelated edits; do not revert or overwrite them.
- Completion requires fresh full verification via `make verify`.

Key decisions:

- Treat the review comment as `valid`: the CLI invocation strings already use `tasks validate`, but the failure messages still reference the retired `validate-tasks` name.
- Fix only the assertion text in the existing test; no production behavior change is needed.
- Keep review-artifact changes constrained to the scoped issue file.

State:

- Completed with unrelated repository verification failure documented.

Done:

- Read the required skill guides for `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, and `testing-anti-patterns`.
- Read `.compozy/tasks/daemon/reviews-003/_meta.md`.
- Read `.compozy/tasks/daemon/reviews-003/issue_003.md` completely before any edits.
- Scanned existing ledgers for cross-agent awareness, with the most relevant context in `2026-04-19-MEMORY-daemon-batch-review.md` and `2026-04-19-MEMORY-daemon-review-fix.md`.
- Inspected `internal/cli/migrate_command_test.go` and confirmed four stale `validate-tasks` assertion messages.
- Updated the four stale assertion messages in `internal/cli/migrate_command_test.go` to `tasks validate`.
- Updated `.compozy/tasks/daemon/reviews-003/issue_003.md` to `valid` with concrete triage reasoning.
- Ran focused verification successfully:
  - `go test ./internal/cli -run 'Test(MigrateCommandPrintsUnmappedTypeSummaryAndValidateFailsUntilFixed|ValidateTasksCommandPassesCommittedACPFixtures)$' -count=1`
- Ran `make verify`, which failed outside this batch in `internal/core/run/ui`:
  - `TestAttachRemoteWithRealSetupHydratesCompletedSnapshotIntoController`
  - `controller state not hydrated from completed snapshot: total=1 jobs=0 completed=0`
  - `session.Wait() error = bubbletea: error opening TTY: bubbletea: could not open TTY: open /dev/tty: device not configured`
- Resolved `.compozy/tasks/daemon/reviews-003/issue_003.md` with the scoped fix and the unrelated verification failure documented.

Now:

- No further code changes are needed in this scoped batch.

Next:

- Prepare the final handoff with precise verification status.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-daemon-issue-003.md`
- `.compozy/tasks/daemon/reviews-003/{_meta.md,issue_003.md}`
- `internal/cli/migrate_command_test.go`
- `git status --short`
- `rg -n "validate-tasks|tasks validate" internal/cli/migrate_command_test.go .compozy/tasks/daemon/reviews-003/issue_003.md`
