# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Implement typed foundation for `reviews watch`: API/core request/result aliases, `[watch_reviews]` config, optional provider watch status, and CodeRabbit current-head status.
- Stop at provider state reporting; do not implement daemon loop, CLI command, route wiring, review-round writing, or git push in this task.

## Important Decisions

- Use task-local session ledger `.codex/ledger/2026-04-30-MEMORY-review-watch-task-01.md`; leave the existing planning ledger read-only.
- Preserve pre-existing dirty worktree changes. Relevant API files already had unrelated workflow-summary edits before task_01 implementation.

## Learnings

- Pre-change signal: `rg -n "ReviewWatchRequest|ReviewWatchResult|WatchStatusProvider|WatchStatus|WatchReviewsConfig|watch_reviews" internal/api internal/core skills/compozy/references/config-reference.md` returned no matches.
- Pre-change targeted baseline passed: `go test ./internal/core/provider ./internal/core/provider/coderabbit ./internal/core/workspace`.
- Provider status states implemented as `pending`, `stale`, and `current_reviewed`; unsupported providers return `provider.ErrWatchStatusUnsupported` through `provider.FetchWatchStatus`.
- CodeRabbit status uses `gh repo view`, `gh api repos/{owner}/{repo}/pulls/{pr}`, and existing pull-request reviews pagination. Latest review selection falls back to review ID when `submitted_at` is absent, so newer pending reviews are not hidden by older submitted reviews.
- `[watch_reviews]` config merges global/workspace values separately from fetch/fix/defaults, validates positive durations, rejects `max_rounds = 0` when `until_clean = true`, requires push remote/branch to be set together, and rejects config-level `auto_push = true` with `defaults.auto_commit = false`.
- Coverage evidence for task-specific packages: provider 80.7%, CodeRabbit 81.2%, workspace 81.6%.
- Final verification evidence: post-commit `make verify` passed; output ended with `All verification checks passed`.

## Files / Surfaces

- Planned surfaces: `internal/api/contract/types.go`, `internal/api/core/interfaces.go`, `internal/core/workspace/config_*`, `internal/core/provider/provider.go`, `internal/core/provider/coderabbit/*`, config reference docs, and focused tests.
- Implemented surfaces also included `internal/core/provider/overlay.go` / `overlay_test.go` for alias delegation, test fakes for the future `ReviewService.StartWatch` method, and `internal/daemon/review_exec_transport_service.go` with an unavailable placeholder for the future service shape.

## Errors / Corrections

- First `make verify` run failed on task-owned test helper loops due `gocritic rangeValCopy`; corrected by indexing `[]provider.ReviewItem` instead of ranging by value.
- Self-review found pending CodeRabbit reviews with empty `submitted_at` could be masked by older submitted reviews; corrected latest-review selection to fall back to review ID when either timestamp is absent.

## Ready for Next Run

- Task 01 implementation and verification are complete in commit `d017bcc` (`feat: add review watch provider contracts`).
- Follow-on tasks can build daemon/CLI watch behavior on `ReviewWatchRequest`, `WatchReviewsConfig`, and `provider.FetchWatchStatus`.
- Tracking/memory files remain uncommitted per repository instruction to keep tracking-only files out of automatic commits.
