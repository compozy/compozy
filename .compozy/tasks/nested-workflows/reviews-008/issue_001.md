---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/cli/reviews_exec_daemon.go
line: 365
severity: high
author: claude-code
provider_ref:
---

# Issue 001: reviews fix discards its positional slug and re-picks a target

## Review Comment

`runReviewWorkflowDaemon` calls `s.maybeCollectInteractiveParams(cmd)` (line 365)
*before* `s.resolveWorkflowNameArg(cmd, args)` (line 368), with no guard.
`maybeCollectInteractiveParams` only short-circuits on `cmd.Flags().NFlag() > 0`
(`internal/cli/state.go:335`); a bare positional argument is invisible to it, and
`applyProjectConfig`/`applyConfig` never mark flags changed.

So `compozy reviews fix nested-workflows` on a TTY opens the interactive form.
`formBuilder.addNameField` (`internal/cli/form.go:269-281`) then pre-selects the
first non-`SelectionBlocked` option into `*target` when it is empty, and
`formInputs.apply` writes that into `state.name`. `resolveWorkflowNameArg` returns
immediately at line 1561 because `s.name != ""`, so `args[0]` is never read.

Failure: the user types a slug, presses Enter through the form, and dispatches an
AI review-fix run that edits and auto-commits against a **different** workflow.

This is a guard asymmetry: the sibling entry points already do it correctly.
`runReviewWatchDaemon` (line 632) guards with
`if len(args) == 0 && strings.TrimSpace(s.name) == ""`, and
`runTaskWorkflow` (`internal/cli/daemon_commands.go:785`) adds `&& s.multiple == ""`.
`reviews fix` (365) and `reviews fetch` (222) both omit it.

Fix: wrap both call sites in the same
`if len(args) == 0 && strings.TrimSpace(s.name) == ""` guard. Add a regression
test asserting that a positional slug survives when `isInteractive` returns true.

## Triage

- Decision: `VALID`
- Root cause: `runReviewWorkflowDaemon` (`reviews_exec_daemon.go:365`) and
  `fetchReviewsDaemon` (`:222`) call `s.maybeCollectInteractiveParams(cmd)`
  unconditionally before `s.resolveWorkflowNameArg(cmd, args)`.
  `maybeCollectInteractiveParams` (`state.go:334-335`) only short-circuits on
  `cmd.Flags().NFlag() > 0`; a bare positional arg is not a flag and
  `applyProjectConfig`/`applyConfig` never mark flags changed, so `NFlag()`
  stays `0`. On a TTY the interactive form therefore opens even when a slug was
  typed. For `fix`, `formBuilder.addNameField` (`form.go:270-300`) pre-selects
  the first non-`SelectionBlocked` review-target option into `*target`
  (`&s.name`); for `fetch`/all kinds the `huh.NewSelect` still binds the
  first-highlighted directory into `s.name` on submit. `resolveWorkflowNameArg`
  (`:1560-1563`) then returns immediately because `s.name != ""`, so `args[0]`
  is never read and the run dispatches (and auto-commits) against a *different*
  workflow than the user named.
- Confirmed the sibling entry points already guard this: `runReviewWatchDaemon`
  (`:632`) wraps the call in `if len(args) == 0 && strings.TrimSpace(s.name) == ""`,
  and `runTaskWorkflow` (`daemon_commands.go:785`) adds `&& s.multiple == ""`.
  `list`/`show` never call `maybeCollectInteractiveParams`, so only `fix` (365)
  and `fetch` (222) were affected — both are in the scoped file.
- Fix: extract the guard into a shared helper
  `collectReviewInteractiveParamsForArgs(cmd, args)` that runs the form only when
  `len(args) == 0 && strings.TrimSpace(s.name) == ""`, and call it from both
  `fix` (365) and `fetch` (222) in place of the bare
  `maybeCollectInteractiveParams`. Same guard semantics as the `watch` sibling;
  a positional slug (or `--name`) now suppresses the form. Behavior-preserving
  for the bare no-arg TTY case (form still opens) and the `--name`/flag cases
  (form still skipped). A shared helper (rather than an inline nested `if` at
  each site) was required to keep `runReviewWorkflowDaemon` under golangci-lint's
  gocyclo limit of 15 — an inline nested guard pushed it to 16.
- Regression test: `TestReviewsFixCommandPositionalSlugSurvivesInteractiveTerminal`
  drives `reviews fix demo` with `isInteractive()==true` and a `collectForm`
  stub that would overwrite `state.name`; it asserts the form is never called
  and the started review run targets `demo` (the typed slug), not the form's
  pick.
- Notes: `make fmt` and `make lint` pass clean (zero issues; the inline nested
  guard tripped gocyclo, resolved by extracting the shared helper above). Fresh
  `go test ./internal/cli/... -race -count=1` = 839 passed, 0 failures, including
  the new regression test — verified it goes red against the pre-fix code
  (form runs, slug becomes `wrong-workflow`) and green after the fix.
- Pre-existing unrelated failure (documented per cy-fix-reviews): the full
  `make verify` reports 24 failures, ALL in `internal/core/plan` and
  `internal/daemon` (packages this batch does not touch), every one caused by
  `rundb: schema too new (db=4 binary=3)`. Those tests open the *shared* global
  `~/.compozy/runs/<TestName>/run.db`, which a concurrent sibling review
  worktree running a newer binary (schema v4) has already written; this
  worktree's binary is at schema v3. It is an environmental collision on shared
  state, independent of this CLI interactive-form guard change (which cannot
  affect rundb schema). Not remediated here: deleting the shared DB would
  disrupt sibling concurrent runs and the live daemon, and it is outside the
  batch scope.
