Goal (incl. success criteria):

- Implement reviews-watch `task_03.md`: expose daemon review watch through API route, Go client, and `compozy reviews watch` CLI with watch flags, inherited review-fix flags, attach/detach/output behavior, auto-push validation, tests, tracking updates, clean `make verify`, and one local commit if feasible.

Constraints/Assumptions:

- Must use workflow memory files under `.compozy/tasks/reviews-watch/memory/`.
- Must not run destructive git commands: `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`.
- Worktree has many pre-existing dirty/untracked files; only task_03-related edits should be staged/committed.
- Task source of truth is `_techspec.md`, `_tasks.md`, `task_03.md`, and ADR-001/003.
- `reviews watch` CLI must start daemon coordinator, not implement its own loop.

Key decisions:

- Reuse existing daemon-backed review run observation: text/detach/stream/UI via `handleStartedTaskRun`, JSON/raw JSON via `streamDaemonWorkflowEvents`.
- Add `commandKindWatchReviews` so watch can inherit `[fetch_reviews]` provider, `[fix_reviews]` output/batching defaults, and `[watch_reviews]` loop defaults without changing existing fetch/fix behavior.
- Extend lean workflow JSON event filtering to include `review.watch_*` events; raw JSON already preserves canonical events.

State:

- Completed. Route/client/CLI/tests are in place, `make verify` passed, tracking was updated, and local commit `b6d4a09` was created.

Done:

- Loaded `cy-workflow-memory`, `cy-execute-task`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, and `no-workarounds` skill guidance.
- Read `AGENTS.md`/`CLAUDE.md`, workflow memory, `_techspec.md`, `_tasks.md`, `task_03.md`, ADR-001/002/003, and relevant prior review-watch ledgers.
- Captured current dirty worktree signal.
- Captured pre-change implementation gap: daemon `ReviewWatchRequest`/`StartWatch` exists, but route/client/CLI watch surface is absent.
- Added route/client/CLI/OpenAPI/contract/tests for review watch.
- Full verification passed: `make verify` ended with `All verification checks passed`.
- Updated task_03 and `_tasks.md` tracking to completed.
- Local commit created: `b6d4a09 feat: expose review watch API and CLI`.

Now:

- Final handoff.

Next:

- None for task_03.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.compozy/tasks/reviews-watch/memory/MEMORY.md`
- `.compozy/tasks/reviews-watch/memory/task_03.md`
- `.compozy/tasks/reviews-watch/task_03.md`
- `.compozy/tasks/reviews-watch/_tasks.md`
- `internal/api/core/{routes.go,handlers.go}`
- `internal/api/client/reviews_exec.go`
- `internal/cli/{root.go,state.go,workspace_config.go,skills_preflight.go,daemon_commands.go,reviews_exec_daemon.go}`
- `openapi/compozy-daemon.json`
- `web/src/generated/compozy-openapi.d.ts`
- Focused tests under `internal/api` and `internal/cli`.
- Focused verification passed: `go test ./internal/api/client ./internal/api/core ./internal/api/httpapi ./internal/cli` and coverage checks for affected packages.
- Full verification passed: `make verify`.
