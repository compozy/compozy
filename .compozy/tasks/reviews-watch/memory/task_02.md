# Task Memory: task_02.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Implement daemon-owned `review_watch` parent run orchestration for task_02 only: duplicate active watch rejection, provider wait/fetch/fix/verify/push state machine, narrow git state/push boundary, structured watch events, required daemon/git coverage, clean `make verify`, tracking updates, and one local commit.
- Keep fetch persistence on the existing review fetch writer path and remediation on existing `StartReviewRun`; do not expand into task_03 CLI/API/client or task_04 extension work unless daemon boundaries require compile-time contracts.

## Important Decisions

- Reuse task_01 `ReviewWatchRequest`, `[watch_reviews]`, and `provider.FetchWatchStatus` contracts.
- Refactor existing fetch internals into reusable item-fetch and round-write steps so watch can avoid empty review directories while still using the existing fetch writer path for non-empty rounds.
- Keep review-watch orchestration inside daemon `RunManager`: the parent run owns provider wait, fetch/import, child `StartReviewRun`, verification, push, duplicate tracking, and event emission.
- Keep production git access behind `ReviewWatchGit`, limited to read-only state commands and `git push <remote> HEAD:<branch>`.

## Learnings

- Pre-change signal: `transportReviewService.StartWatch` currently returns `review_service_unavailable`; daemon has no `review_watch` run mode/coordinator yet.
- Current `FetchReviewsWithRegistryDirect` writes a `reviews-NNN` directory even when the provider returns zero items, so clean watch detection needs a no-write item fetch step before persistence.
- Focused watch coverage evidence: `review_watch.go` 83.3% and `review_watch_git.go` 96.4% via `/tmp/review-watch.cover`.
- Focused test evidence: `go test ./internal/core ./internal/daemon ./pkg/compozy/events/...` passes after coordinator, git boundary, service, and event coverage.
- Full verification evidence: `make verify` passed after final code/test changes; output ended with `All verification checks passed`.

## Files / Surfaces

- `internal/core/fetch.go`
- `internal/daemon/run_manager.go`
- `internal/daemon/review_exec_transport_service.go`
- `internal/daemon/review_watch.go`
- `internal/daemon/review_watch_git.go`
- `internal/daemon/review_watch_test.go`
- `internal/daemon/review_watch_git_test.go`
- `internal/daemon/review_exec_transport_service_test.go`
- `internal/daemon/run_manager_test.go`
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/event_test.go`
- `pkg/compozy/events/docs_test.go`
- `pkg/compozy/events/kinds/review.go`
- `pkg/compozy/events/kinds/payload_compat_test.go`
- `docs/events.md`
## Errors / Corrections

- Added explicit event documentation and payload compatibility coverage for all `review.watch_*` public event kinds after the initial coordinator pass.
- Added focused coverage for unresolved-round verification, auto-push target validation, provider/fetch/git failures, runtime override sanitization, duplicate reservation release, and real git command wrapper/parsers.
- Refactored the coordinator after `make verify` lint caught unchecked error paths plus funlen/gocyclo pressure; error emission/cancel paths now propagate failures explicitly.
- Fixed race-test synchronization for duplicate-watch rejection by holding the provider wait with a longer timeout and asserting duplicate reservation directly instead of racing a transient running-row state.

## Ready for Next Run

- Latest task-local race subset passes: `go test -race ./internal/daemon -run 'TestRunManagerReviewWatch|TestReviewWatch|TestTransportReviewServiceStartWatch' -count=1`.
- Task tracking was marked completed after clean `make verify` and self-review.
- Required local commit created: `3db28de feat: implement daemon review watch coordinator`.
- Workflow tracking, workflow memory, and ledger files were intentionally left out of the automatic commit per tracking-file policy.
