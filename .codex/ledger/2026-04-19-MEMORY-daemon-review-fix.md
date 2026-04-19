Goal (incl. success criteria):

- Resolve the scoped CodeRabbit batch issue for PR `116`, round `003`, by fixing the `daemon start` format-validation inconsistency in `internal/cli/daemon.go`.
- Success means: `daemon start --foreground --format <invalid>` fails consistently with the detached path, regression coverage exists, `.compozy/tasks/daemon/reviews-003/issue_001.md` is triaged and closed correctly, and fresh `make verify` passes.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the batched review execution contract.
- Required skills loaded/read this session: `cy-fix-reviews`, `cy-final-verify`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `golang-pro`.
- Scope is limited to `internal/cli/daemon.go`, needed test coverage in the existing CLI daemon tests, and `.compozy/tasks/daemon/reviews-003/issue_001.md`.
- Do not touch other review issue files or unrelated dirty worktree changes.
- Completion requires fresh full verification via `make verify`.

Key decisions:

- Treat the review issue as valid against the tracked code at batch start: `daemonStartState.run` validated `s.outputFormat` only after the detached-start branch, leaving `foreground` and `internalChild` paths unvalidated.
- Fix at the root cause by validating `s.outputFormat` before the shared early return instead of duplicating branch-local checks.
- Add regression coverage in `internal/cli/daemon_commands_test.go` so both early-return paths are guarded.

State:

- Completed.

Done:

- Read the required skill guides and the batch context in `.compozy/tasks/daemon/reviews-003/_meta.md`.
- Read `.compozy/tasks/daemon/reviews-003/issue_001.md` completely before editing.
- Scanned existing daemon-related ledgers for cross-agent awareness, with the most relevant prior context in `2026-04-18-MEMORY-daemon-command-surface.md` and `2026-04-18-MEMORY-daemon-memory-research.md`.
- Inspected `internal/cli/daemon.go` and confirmed the review finding was real in the tracked code at batch start.
- Inspected the existing foreground daemon-start tests in `internal/cli/daemon_commands_test.go`.
- Implemented the root-cause fix in `internal/cli/daemon.go` by validating `s.outputFormat` before the foreground/internal-child early return.
- Added regression coverage in `internal/cli/daemon_commands_test.go` for invalid `--format` values on both `--foreground` and `--internal-child`.
- Ran focused verification successfully:
  - `go test ./internal/cli -run 'TestDaemonStartCommand(ForegroundUsesDaemonRunner|RejectsInvalidFormatBeforeEarlyReturn)$' -count=1`
- Ran the full repository gate successfully:
  - `make verify`
- Resolved `.compozy/tasks/daemon/reviews-003/issue_001.md` with the final root cause, production fix, regression coverage, and verification evidence.
- Reconciled conflicting in-scope edits to the issue artifact and ledger by restoring the correct final state based on the actual code diff and fresh verification evidence.

Now:

- No technical work remains; prepare the final verified handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-daemon-review-fix.md`
- `.compozy/tasks/daemon/reviews-003/{_meta.md,issue_001.md}`
- `internal/cli/{daemon.go,daemon_commands_test.go}`
- `git status --short`
- `rg`
- `sed -n`
- `go test ./internal/cli -run 'TestDaemonStartCommand(ForegroundUsesDaemonRunner|RejectsInvalidFormatBeforeEarlyReturn)$' -count=1`
- `make verify`
