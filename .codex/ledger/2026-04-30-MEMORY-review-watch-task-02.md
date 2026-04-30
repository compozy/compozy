Goal (incl. success criteria):

- Implement reviews-watch `task_02.md`: daemon-owned `review_watch` parent run mode, coordinator state machine, duplicate-watch protection, narrow git push boundary, structured watch events, and required daemon/git tests.
- Success requires task-specific tests plus full `make verify`, tracking updates, memory updates, and one local commit.

Constraints/Assumptions:

- Must follow workflow memory files under `.compozy/tasks/reviews-watch/memory/`.
- Must preserve existing review fetch/write and `StartReviewRun` remediation behavior rather than creating a second fix pipeline.
- Must not run destructive git commands: `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`.
- Production watch git boundary must only read git state and run `git push <remote> HEAD:<branch>`.
- Scope is task_02; CLI/API/client/provider/extension work belongs to other tasks unless already required by daemon boundaries.

Key decisions:

- Reuse task_01 `ReviewWatchRequest`, `provider.FetchWatchStatus`, and `WatchReviewsConfig` contracts.
- Split the existing fetch implementation into "fetch normalized items" and "write round" steps so watch can detect reviewed-clean without creating an empty `reviews-NNN` directory while still using the existing writer path for non-empty rounds.

State:

- Completed. Implemented coordinator/git boundary/events/tests; `make verify` passed; local commit `3db28de` created.

Done:

- Read `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `systematic-debugging`, `no-workarounds`, and `testing-anti-patterns` skill instructions.
- Read repository guidance (`AGENTS.md`/`CLAUDE.md`), workflow memory, task_02, `_techspec.md`, `_tasks.md`, and ADR-001 through ADR-003.
- Inspected task_01 contracts and daemon run manager/review service/test patterns.
- Refactored review fetch into no-write item fetch plus existing writer reuse.
- Added daemon `review_watch` run mode, duplicate active-watch tracking, parent coordinator, child `StartReviewRun` orchestration, verification, push boundary, and watch event emission.
- Added git state/push runner constrained to read-only state commands plus `git push <remote> HEAD:<branch>`.
- Added daemon/git tests for clean exit, round persistence, duplicate watch, cancellation, unchanged HEAD, push/repeat, terminal failures, and stream visibility.
- Targeted daemon tests for the new coordinator/git runner passed before adding the transport service coverage.
- Added event docs/tests for all `review.watch_*` kinds and `ReviewWatchPayload` JSON compatibility.
- Focused verification passed: `go test ./internal/core ./internal/daemon ./pkg/compozy/events/...`.
- Focused watch coverage passed after refactor: `review_watch.go` 83.3%, `review_watch_git.go` 96.4% from `/tmp/review-watch.cover`.
- Fixed Go lint findings from the first `make verify` attempt by splitting the coordinator and checking all event/cancel errors.
- Fixed duplicate-watch race-test synchronization; watch/race subset passes.
- Full verification passed: `make verify` completed with `All verification checks passed`.
- Updated task_02 and `_tasks.md` tracking to completed after verification and self-review.
- Local commit created: `3db28de feat: implement daemon review watch coordinator`.

Now:

- Final handoff.

Next:

- None for task_02.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.compozy/tasks/reviews-watch/task_02.md`
- `.compozy/tasks/reviews-watch/_techspec.md`
- `.compozy/tasks/reviews-watch/memory/MEMORY.md`
- `.compozy/tasks/reviews-watch/memory/task_02.md`
- `internal/core/fetch.go`
- `internal/daemon/run_manager.go`
- `internal/daemon/review_exec_transport_service.go`
- `pkg/compozy/events/kinds/review.go`
- `pkg/compozy/events/event.go`
- `internal/daemon/review_watch.go`
- `internal/daemon/review_watch_git.go`
- `internal/daemon/review_watch_test.go`
- `internal/daemon/review_watch_git_test.go`
- `internal/daemon/review_exec_transport_service_test.go`
- `docs/events.md`
- `pkg/compozy/events/docs_test.go`
- `pkg/compozy/events/event_test.go`
