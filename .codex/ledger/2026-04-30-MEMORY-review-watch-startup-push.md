Goal (incl. success criteria):

- Implement the accepted review-watch startup auto-push fix so `cy reviews watch --auto-push` reconciles already-committed local fixes before waiting on CodeRabbit.
- Success means daemon regression tests cover startup unpushed commits, push failure, and hook veto; focused tests and `make verify` pass.

Constraints/Assumptions:

- Do not run destructive git commands: `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`.
- Preserve unrelated dirty worktree changes.
- Accepted plan is persisted at `.codex/plans/2026-04-30-review-watch-startup-auto-push.md`.
- Use root-cause fix; no arbitrary delay or symptom workaround.
- `--auto-push` permits pushing already-committed local branch state at startup when `UnpushedCommits > 0`.

Key decisions:

- Add startup reconciliation before the first provider wait.
- Reuse existing `review.watch_push_*` events with `round: 0`.
- Reuse the existing pre-push hook for startup push; hook veto stops the parent run explicitly.

State:

- Complete; verification passed.

Done:

- Investigated root cause from run events, provider status, and local DB evidence.
- Persisted accepted plan.
- Added startup auto-push reconciliation before provider wait.
- Added daemon regressions for startup push, startup push failure, and startup pre-push veto.
- Updated `watch_reviews` docs for startup reconciliation and `round = 0` push events.
- Focused daemon tests passed.
- Focused daemon race tests passed.
- Full `make verify` passed.

Now:

- Ready to report implementation and verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-review-watch-startup-auto-push.md`
- `.codex/ledger/2026-04-30-MEMORY-review-watch-startup-push.md`
- `internal/daemon/review_watch.go`
- `internal/daemon/review_watch_test.go`
- `skills/compozy/references/config-reference.md`
- `go test ./internal/daemon -run 'TestRunManagerReviewWatch.*(Startup|Push|Provider|Manual|Failure)' -count=1`
- `go test -race ./internal/daemon -run 'TestRunManagerReviewWatch.*(Startup|Push|Provider|Manual|Failure)' -count=1`
- `make verify`
