Goal (incl. success criteria):

- Fix workflow sync so review `_meta.md` is no longer considered operational input.
- Success: sync handles empty `reviews-NNN` dirs and legacy issue files without `_meta.md`, projects review rounds from issue files only, targeted tests pass, and final `make verify` passes if unrelated workspace state allows.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE rules: no destructive git commands, use required skills, `make verify` before completion.
- User selected policy: project by directory + issue files; ignore `_meta.md`; allow empty provider/pr; ignore empty review dirs.
- Worktree already has unrelated dirty files in review/fetch areas; do not touch or revert unrelated changes.
- Accepted Plan Mode plan persisted at `.codex/plans/2026-04-30-sync-review-rounds-no-meta.md`.

Key decisions:

- `collectReviewRounds` should not call `reviews.SnapshotRoundMeta`.
- Empty `reviews-NNN` directories should be skipped during sync.
- Legacy issue files without round metadata should still sync as review issues with blank provider/pr.
- `_meta.md` remains readable only via explicit legacy/migration helpers.

State:

- Implementation complete; focused and full verification passed.

Done:

- Root cause traced to `collectReviewRounds -> reviews.SnapshotRoundMeta -> ReadLegacyRoundMeta`.
- Confirmed `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/badges-design/reviews-002` is empty.
- Persisted accepted plan and created ledger.
- Refactored sync review-round collection to use issue files only, skip empty review dirs, and validate present frontmatter consistency.
- Removed runtime `SnapshotRoundMeta` fallback to legacy `_meta.md`.
- Relaxed `globaldb` review-round validation to allow blank provider.
- Added/updated focused regression tests for sync, review metadata snapshots, and globaldb validation.
- Targeted `go test ./internal/core ./internal/core/reviews ./internal/store/globaldb` passed.
- Full `make verify` passed: frontend lint/typecheck/test/build, Go fmt/lint/tests/build, Go build, and Playwright e2e.
- Final status includes unrelated/concurrent modified files outside this task; left untouched.

Now:

- Final handoff.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-sync-review-rounds-no-meta.md`
- `.codex/ledger/2026-04-30-MEMORY-sync-review-rounds.md`
- `internal/core/sync.go`
- `internal/core/sync_test.go`
- `internal/core/reviews/store.go`
- `internal/core/reviews/store_test.go`
- `internal/store/globaldb/sync.go`
- `internal/store/globaldb/sync_test.go`
- Targeted test: `go test ./internal/core ./internal/core/reviews ./internal/store/globaldb`
- Final verification: `make verify`
