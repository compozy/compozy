# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State

## Shared Decisions

- Task 01 established provider watch states as `pending`, `stale`, and `current_reviewed`; unsupported providers surface through `provider.ErrWatchStatusUnsupported`.
- Future clean detection should require `current_reviewed` provider status for the PR head plus an empty normalized fetch result.
- `[watch_reviews]` config owns loop-only values; child fetch/fix/runtime behavior remains under `[fetch_reviews]`, `[fix_reviews]`, and `[defaults]`.
- Task 02 implemented `review_watch` as a daemon-owned parent run that emits `review.watch_*` events and launches existing daemon review fix runs as children.
- Task 02 narrowed production git behavior to read-only state inspection plus `git push <remote> HEAD:<branch>` after zero unresolved issues and HEAD advancement verification.
- Task 03 exposed the daemon review watch coordinator through `POST /api/reviews/:slug/watch`, the Go API client, and `compozy reviews watch [slug]`; the CLI still only serializes intent and observes the daemon parent run.
- Task 04 added Go/TypeScript SDK hooks `review.watch_pre_round`, `review.watch_post_round`, `review.watch_pre_push`, and `review.watch_finished`; only pre-round and pre-push are mutable, and daemon validation rejects immutable provider/head/status mutations.
- Review-watch parent runs force executable extension dispatch enabled, but provider-current clean detection remains daemon-owned and runs before `review.watch_pre_round`.

## Shared Learnings

- CodeRabbit pending reviews may lack `submitted_at`; latest-review selection must not rely on timestamps alone.
- `auto_push=true` forces child `auto_commit=true`; an explicit `runtime_overrides.auto_commit=false` is rejected at watch start.
- CLI `--auto-push` forces `auto_commit=true` before daemon bootstrap and rejects explicit `--auto-push --auto-commit=false` with `invalid_watch_request`.
- A `review.watch_pre_push` hook can stop a push only with `push=false` plus a non-empty `stop_reason`; the stopped reason is surfaced through `review.watch_post_round` and `review.watch_finished`.

## Open Risks

## Handoffs

- Task 02 can use `provider.FetchWatchStatus` and `WatchReviewsConfig` instead of adding another provider/config stack.
- Task 03 can expose daemon `StartWatch`; the daemon service boundary already accepts `ReviewWatchRequest` and returns the parent run.
- Task 04 can rely on the API/client/CLI watch surface and OpenAPI `ReviewWatchRequest` schema being present.
- Task 02 implementation commit is `3db28de`.
- Task 03 implementation commit is `b6d4a09`.
- Task 04 implementation commit is `3b60eb6`.
