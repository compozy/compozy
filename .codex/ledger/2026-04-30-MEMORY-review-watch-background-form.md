Goal (incl. success criteria):

- Implement accepted plan for `compozy reviews watch`: form only for missing input, no ACP cockpit UI during watch execution, background message by default.
- Success means focused CLI regressions pass and final verification is attempted with `make verify`.

Constraints/Assumptions:

- Accepted plan persisted at `.codex/plans/2026-04-30-review-watch-background-form.md`.
- Do not run destructive git commands: `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`.
- Preserve `--stream`, `--format json`, and `--format raw-json` as explicit observation modes.
- Limit implementation to CLI/form behavior and tests; ignore unrelated dirty worktree files.
- Use root-cause fix; no workaround or UI suppression after attach starts.

Key decisions:

- `reviews watch` default presentation should be detach, even on interactive terminals.
- `--ui`, `--attach ui`, and explicit `--tui=true` should fail before daemon bootstrap.
- No-arg `reviews watch` should call the existing huh form before workflow slug validation.

State:

- Implementation complete; focused tests and full `make verify` passed.

Done:

- Read relevant skills and current CLI/form/watch code during planning.
- Persisted accepted plan.
- Added failing regressions for no-arg form, interactive default background, and watch form workflow selection.
- Patched `reviews watch` to collect form input before slug validation.
- Added watch-specific presentation handling and started-run output with no UI attach path.
- Added watch form workflow directory selection.
- Updated existing review-watch output/UI tests.
- Focused `go test ./internal/cli -run 'TestReviewsWatch' -count=1` passed.
- `make verify` failed at lint: `internal/cli/reviews_exec_daemon.go:395:1 cyclomatic complexity 17 of runReviewWatchDaemon is high (> 15)`.
- Extracted `observeStartedReviewWatchRun` to reduce `runReviewWatchDaemon` complexity.
- Focused `go test ./internal/cli -run 'TestReviewsWatch|Test.*Form|TestResolve.*Presentation' -count=1` passed.
- `go test ./internal/cli -count=1` passed.
- Full `make verify` passed: frontend lint/typecheck/test/build, Go fmt/lint/test/build, and frontend e2e.

Now:

- Ready to report final verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-review-watch-background-form.md`
- `.codex/ledger/2026-04-30-MEMORY-review-watch-background-form.md`
- Planned: `internal/cli/reviews_exec_daemon.go`, `internal/cli/form.go`, `internal/cli/form_test.go`, `internal/cli/reviews_exec_daemon_additional_test.go`
