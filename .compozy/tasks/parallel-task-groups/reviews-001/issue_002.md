---
provider: manual
pr:
round: 1
round_created_at: 2026-07-23T18:09:25Z
status: resolved
file: internal/cli/daemon_commands.go
line: 1101
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: Dirty-tree launch warning (ADR-010 / US-001.EC-3 / R3) unimplemented

## Review Comment

`task_06.md` is marked `status: completed`, but its explicit **MUST** requirement
R3 — "add a dirty-tree check to `preflightParallelWorktreeMode`: a non-empty
`git status --porcelain` WARNS (to stderr) and PROCEEDS" — is not implemented.

Verified: `preflightParallelWorktreeMode` (`internal/cli/daemon_commands.go:1101-1147`)
checks only (a) empty workspace root, (b) workspace inside the Compozy-managed
worktree root, and (c) detached HEAD. There is no `git status --porcelain`
invocation anywhere in the non-test CLI code, and the signature was never
threaded with the `cmd`/writer R3 requires. The contracted tests `UT-071`
(uncommitted → warn + return nil) and `E2E-009` (warning printed, run proceeds,
uncommitted changes remain in checkout) are absent, and `UT-070` (clean → no
warn) has no dedicated unit test.

Failure scenario: a user with uncommitted WIP runs
`compozy tasks run --multiple init/TG-001,init/TG-002 --parallel-task-groups`.
Branches are cut from the base commit, silently excluding the WIP, and the user
gets no signal that this happened — ADR-010 names communicating exactly this as
the load-bearing trade-off of the warn-and-proceed decision. No data is lost
(the WIP stays in the checkout), so the impact is an absent advisory plus a
test-contract gap, but the behavior nonetheless deviates from a MUST requirement
in a task that claims completion.

Suggested fix: thread `cmd` into `preflightParallelWorktreeMode`, run
`git status --porcelain`, and on non-empty output write a warning to
`cmd.ErrOrStderr()` (mirroring `writeTaskRunConcurrencyWarning`, treating the
write error as advisory) before returning nil. Add UT-070/UT-071 and E2E-009.

## Triage

- Decision: `VALID`
- Root cause: `preflightParallelWorktreeMode` (`internal/cli/daemon_commands.go:1101`)
  performs only the workspace-root, worktree-containment, and detached-HEAD
  checks. There is no `git status --porcelain` invocation anywhere in the
  non-test CLI code, so R3 (ADR-010 / US-001.EC-3) — "a non-empty
  `git status --porcelain` WARNS to stderr and PROCEEDS" — is unimplemented, and
  the function was never threaded with a writer to emit the warning. Confirmed by
  reading the function body end to end and grepping the package for
  `status --porcelain` (no hits). The contracted UT-070 (clean → no warn),
  UT-071 (uncommitted → warn + return nil), and E2E-009 (warning printed, run
  proceeds, changes remain) are all absent.
- Fix approach:
  1. Thread `cmd *cobra.Command` into `preflightParallelWorktreeMode` (both call
     sites at `:830` and `:978` already hold `cmd`).
  2. After the branch check passes, run `git status --porcelain` via the existing
     `runTaskRunGitPreflight`; on non-empty output write an advisory warning to
     `cmd.ErrOrStderr()` (mirroring `writeTaskRunConcurrencyWarning`, treating a
     write error as advisory) and return nil. A status-command error is treated
     as advisory (proceed without warning) since the branch preflight already
     validated the repo.
  3. Add UT-070, UT-071 (unit, real temp git repo) and E2E-009 (full CLI →
     in-process daemon parallel run with an uncommitted file that must survive).
- Out-of-scope file touched: `internal/cli/testdata/tasks_run_help.golden` is not
  edited by this issue; see issue_007 (help-text golden) for the only testdata
  change in this batch.
- Notes: warn-and-proceed applies in `--dry-run` too, because branches are still
  cut in dry-run (verified against the existing parallel E2E dry-run tests).
- Verification: `make fmt` PASS, `make lint` PASS (0 issues), full Go suite
  `gotestsum -race ./...` PASS (5432 tests, 0 failures — includes the new
  UT-070/UT-071/E2E-009), `go-build` + `verify-extensions` + `frontend-verify`
  PASS. Two environmental flakes were observed and ruled out as unrelated: (1) a
  first run hit `signal: segmentation fault` from `git worktree add` /
  `git merge --squash` in two parallel-worktree tests, which both pass in
  isolation (`go test -race -run TestTasksRunParallelTaskGroupsEndToEndFaultReporting`
  and `...RecoveryRecoversFailingTask`); (2) the final `frontend-e2e` Playwright
  step failed in its `global.setup.ts` because `compozy daemon start` did not
  publish `daemon.json` (daemon-readiness race in the shared review workspace).
  Neither touches the changed CLI code path.
