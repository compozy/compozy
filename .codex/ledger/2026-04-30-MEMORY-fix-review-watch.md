Goal (incl. success criteria):

- Fix `reviews watch` premature clean detection so the daemon waits for CodeRabbit to finish processing the current PR head before fetching/declaring clean.
- Success means provider/daemon regressions cover the PR #133 timing failure, focused tests pass, and full `make verify` passes.

Constraints/Assumptions:

- Accepted plan persisted at `.codex/plans/2026-04-30-fix-review-watch-premature-clean.md`.
- Must not run destructive git commands: `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`.
- Must use root-cause fix; no arbitrary delay-only workaround.
- Use `systematic-debugging`, `no-workarounds`, `golang-pro`, `testing-anti-patterns`, and `cy-final-verify`.

Key decisions:

- CodeRabbit `WatchStatus` must gate `current_reviewed` on the newest CodeRabbit commit status for the PR head being success, not merely on an existing submitted review.
- Daemon quiet period must be provider-settle based and must recheck provider status before fetching.
- Non-auto-push watch loops must not mark clean against an old PR head after a child fixes issues locally.

State:

- Provider, daemon, docs, and regression edits are complete; focused regressions and full `make verify` passed.

Done:

- Investigated persisted run events for PR #133 watch run.
- Confirmed parent run completed clean at `2026-04-30T21:10:15Z`, before later CodeRabbit `CHANGES_REQUESTED` at `2026-04-30T21:19:28Z`.
- Confirmed GitHub commit statuses for `acdd696f` had CodeRabbit success `Review approved`, then pending `Review in progress`, then success `Review completed`.
- Persisted accepted implementation plan.
- Added optional provider-status metadata to `provider.WatchStatus`.
- Added CodeRabbit commit-status gating before `current_reviewed`.
- Updated daemon waiting to require expected PR head, provider-settle quiet period, and status recheck before fetch.
- Removed post-push-only quiet wait.
- Added regression tests for CodeRabbit status gating, provider settle behavior, and manual-push head waiting.
- Updated `[watch_reviews].quiet_period` docs.
- Ran `gofmt` on touched Go files.
- Focused tests passed: `go test ./internal/core/provider ./internal/core/provider/coderabbit ./internal/daemon -run 'TestWatchStatus|TestLatestCodeRabbitCommitStatus|TestRunManagerReviewWatch' -count=1`.
- Focused race tests passed: `go test -race ./internal/core/provider ./internal/core/provider/coderabbit ./internal/daemon -run 'TestWatchStatus|TestLatestCodeRabbitCommitStatus|TestRunManagerReviewWatch' -count=1`.
- Full verification passed: `make verify`.

Now:

- Ready to report final verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-fix-review-watch-premature-clean.md`
- `.codex/ledger/2026-04-30-MEMORY-fix-review-watch.md`
- `internal/core/provider/provider.go`
- `internal/core/provider/coderabbit/coderabbit.go`
- `internal/core/provider/coderabbit/coderabbit_test.go`
- `internal/daemon/review_watch.go`
- `internal/daemon/review_watch_test.go`
- `skills/compozy/references/config-reference.md`
