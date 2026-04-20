Goal (incl. success criteria):

- Resolve the scoped CodeRabbit batch item `.compozy/tasks/daemon/reviews-005/issue_001.md` for PR `116`, round `005`.
- Success means: the attach/cancel timing review comment is triaged against the current `internal/cli/run_observe.go`, any real defect is fixed at the root, regression coverage exists, fresh `make verify` passes, and the batch is ready for manual review without unrelated edits.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the batched review execution contract.
- Required skills read this session: `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`.
- Only `.compozy/tasks/daemon/reviews-005/issue_001.md` is in scope for review-artifact edits.
- Code scope centers on `internal/cli/run_observe.go`; test file edits are allowed where needed to validate the fix.
- Current worktree state only shows `.compozy/tasks/daemon/reviews-005/` as untracked; do not disturb unrelated files.
- Completion requires fresh full verification via `make verify`.

Key decisions:

- Treat the review comment as `valid` against the current code: `loadUIAttachSnapshot` still uses `time.Sleep` for warmup polling, and the attach snapshot/cancel timeouts remain hardcoded constants with no override path for callers.
- Fix at the root by introducing internal functional timing options for the run-observe helpers, threading those options through attach/cancel operations, and replacing the warmup sleep loop with a `ticker`/`select` loop that observes `ctx.Done()` promptly.
- Keep production edits minimal and constrained to the observe helpers plus targeted regression coverage in the existing CLI daemon tests.

State:

- Completed.

Done:

- Read the required skill guides for `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, `systematic-debugging`, `no-workarounds`, and `testing-anti-patterns`.
- Read `.compozy/tasks/daemon/reviews-005/_meta.md`.
- Read `.compozy/tasks/daemon/reviews-005/issue_001.md` completely before any code edits.
- Scanned daemon-related ledgers for cross-agent awareness.
- Inspected `internal/cli/run_observe.go` and confirmed the warmup loop still sleeps and the attach/cancel timings are still hardcoded at the call sites.
- Inspected the existing attach-related tests in `internal/cli/daemon_commands_test.go`.
- Updated `.compozy/tasks/daemon/reviews-005/issue_001.md` to `valid` with concrete root-cause triage before editing code.
- Implemented internal timing options in `internal/cli/run_observe.go` and replaced the warmup `time.Sleep` loop with a `ticker`/`select` loop that observes `ctx.Done()`.
- Added regression coverage in `internal/cli/daemon_commands_test.go` for prompt warmup cancellation plus attach/cancel timing override wiring.
- Ran focused verification successfully:
  - `go test ./internal/cli -run 'Test(DefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit|NewAttachStartedCLIRunUIUsesConfiguredOwnedRunCancelTimeout|LoadUIAttachSnapshotWaitsForJobsWhenInitialSnapshotIsEmpty|LoadUIAttachSnapshotReturnsPromptlyWhenContextCanceledDuringWarmup|NewAttachCLIRunUIDisablesWarmupWhenConfiguredTimeoutIsZero)$' -count=1`
- Ran `make verify` successfully:
  - formatting passed
  - lint passed with `0 issues`
  - `DONE 2415 tests, 1 skipped in 42.175s`
  - build succeeded
- Updated `.compozy/tasks/daemon/reviews-005/issue_001.md` to `status: resolved` with final resolution and verification evidence.
- Recorded that `make verify` also left an unrelated tracked diff in `internal/cli/workspace_config_test.go`; per workspace policy, it remains untouched because it is outside this batch.

Now:

- No technical work remains; prepare the final verified handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-daemon-review-005.md`
- `.compozy/tasks/daemon/reviews-005/{_meta.md,issue_001.md}`
- `internal/cli/{run_observe.go,daemon_commands_test.go}`
- `git status --short`
- `rg -n "attachRemoteCLIRunUI|loadUIAttachSnapshot|cancelOwnedDaemonRun" internal/cli -g'*.go'`
- `sed -n`
- `go test ./internal/cli -run 'Test(DefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit|NewAttachStartedCLIRunUIUsesConfiguredOwnedRunCancelTimeout|LoadUIAttachSnapshotWaitsForJobsWhenInitialSnapshotIsEmpty|LoadUIAttachSnapshotReturnsPromptlyWhenContextCanceledDuringWarmup|NewAttachCLIRunUIDisablesWarmupWhenConfiguredTimeoutIsZero)$' -count=1`
- `make verify`
