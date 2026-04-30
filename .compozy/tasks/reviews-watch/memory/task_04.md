# Task Memory: task_04.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Implement task_04: review watch extension hooks in Go and TypeScript SDKs, operator documentation/reference updates, hook/coordinator QA coverage, `compozy tasks validate --name reviews-watch`, full `make verify`, tracking updates, and one local commit after clean verification.
- Key invariants from TechSpec/ADRs: hooks may stop/veto or adjust allowed round/push options only; hooks must not bypass provider-current clean detection; auto-push safety boundaries and non-destructive git policy remain intact.

## Important Decisions

- Go and TypeScript SDKs expose the four watch hooks with parity; only `review.watch_pre_round` and `review.watch_pre_push` are mutable.
- Daemon watch parent runs force executable extension dispatch enabled so hooks run from daemon-owned orchestration.
- Hook mutability enforcement stays allowlist-based: pre-round can adjust only nitpicks/runtime overrides/batching/continue/stop reason, and pre-push can adjust only remote/branch/push/stop reason.
- Provider-current clean detection remains daemon-owned: pre-round hooks run after provider watch status, and immutable provider/status/head fields are rejected if a hook attempts to mutate them.

## Learnings

- Baseline signal before implementation: the four required watch hook names were absent from SDK, daemon, host manifest, and docs surfaces (`rg` exited 1).
- Existing docs already documented `review.watch_*` event names; Task 04 added the missing hook payload, CLI usage, and auto-push safety reference material.
- Full verification required two root-cause lint fixes after implementation: post-round hook dispatch errors are now joined into the original failure, and repeated failed-round hook emission was extracted to keep `runReviewWatchRound` under the repository complexity limit.
- Required validation evidence captured on 2026-04-30: `go run ./cmd/compozy tasks validate --name reviews-watch` exited 0 with `all tasks valid (4 scanned)`; `make verify` exited 0 with frontend checks, Go lint `0 issues`, Go tests `DONE 2903 tests, 3 skipped`, Go build, Playwright `5 passed`, and `All verification checks passed`.

## Files / Surfaces

- Updated: `sdk/extension`, `sdk/extension-sdk-ts`, `internal/core/extension/manifest.go`, `internal/daemon/review_watch.go`, `internal/daemon/review_watch_hooks.go`, `internal/daemon/review_watch_test.go`.
- Updated docs/references: `skills/compozy/references/config-reference.md`, `docs/extensibility/hook-reference.md`, `docs/extensibility/capability-reference.md`, `docs/events.md`, `README.md`.
- Updated tracking: `.compozy/tasks/reviews-watch/task_04.md`, `.compozy/tasks/reviews-watch/_tasks.md`.

## Errors / Corrections

- First `make verify` failure: `errcheck` rejected ignored `review.watch_post_round` observer wait errors; fixed by joining hook dispatch errors into the failed-round error path.
- Second `make verify` failure: `gocyclo` rejected `runReviewWatchRound` complexity after hook wiring; fixed by extracting `dispatchReviewWatchFailedRoundHook`.

## Ready for Next Run

- Task 04 implementation and verification are complete.
- Local commit created: `3b60eb6 feat: add review watch extension hooks`.
- Task tracking and memory files remain unstaged/uncommitted by policy; unrelated dirty workspace files also remain untouched.
