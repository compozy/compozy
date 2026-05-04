# Task Memory: task_03.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Implement task_03 reviews watch API/client/CLI surface by exposing daemon `StartWatch` through `POST /api/reviews/:slug/watch`, a typed Go client method, and `compozy reviews watch [slug]`.
- Success requires request serialization, error status mapping, auto-push/auto-commit validation, reuse of daemon run observation for attach/detach/json/raw-json/UI/stream modes, focused tests, `make verify`, tracking updates, and a local commit after clean verification if the worktree permits isolated staging.

## Important Decisions

- Use task_02 daemon coordinator as the only execution source; CLI must validate and serialize intent, then observe the returned daemon run.
- Preserve pre-existing dirty worktree changes and stage only files touched for task_03.
- Add a dedicated `commandKindWatchReviews` to apply `[watch_reviews]` loop defaults while reusing `[fetch_reviews]` provider and `[fix_reviews]` output/batching defaults.
- Reuse existing daemon observation helpers for watch runs instead of adding a watch-specific foreground loop.
- Include `review.watch_*` events in lean workflow JSON output; raw JSON already preserves canonical parent-run events.

## Learnings

- Shared memory says task_02 commit `3db28de` added daemon `StartWatch`, duplicate-watch protection, events, and daemon service boundary accepting `ReviewWatchRequest`.
- ADR-003 requires `--auto-push` to force child `auto_commit=true` while rejecting explicit `--auto-commit=false` before daemon bootstrap.
- Baseline inspection found watch contracts and daemon service methods present, but no API route, handler method, client `StartReviewWatch`, or `reviews watch` command registration yet.
- Focused affected-package tests pass for API client/core/httpapi and CLI after adding the route, client method, command surface, OpenAPI contract expectations, and output-mode coverage.
- Full repository verification passed with `make verify` after fixing local lint/contract inventory findings.

## Files / Surfaces

- Touched task_03 surfaces: `internal/api/core`, `internal/api/httpapi`, `internal/api/client`, `internal/cli`, `openapi/compozy-daemon.json`, `web/src/generated/compozy-openapi.d.ts`, and focused tests.
- CLI watch uses a new `commandKindWatchReviews`, applies `[watch_reviews]` loop defaults, reuses review fix batching/runtime flags, forces `auto_commit=true` for auto-push, and observes daemon runs through existing attach/detach/JSON/raw JSON helpers.

## Errors / Corrections

- JSON/raw-json watch output test originally closed the fake stream error channel too early, causing nondeterministic early exit before queued events drained. The fixture now relies on the terminal run event to end the stream, matching real daemon behavior.
- First full verification failed on task-local lint: `applyProjectConfig` exceeded funlen and one watch help example exceeded 120 chars. Extracted watch config application and wrapped the example.
- Second full verification failed because the core route inventory lacked `POST /api/reviews/:slug/watch`. Added the route contract entry with `RunResponse` and `TimeoutLongMutate`.

## Ready for Next Run

- Implementation, verification, tracking updates, and local commit are complete.
- Commit: `b6d4a09 feat: expose review watch API and CLI`.
