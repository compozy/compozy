Goal (incl. success criteria):

- Remediate review batch `reviews-watch` round `004` for PR `133`.
- Success means both scoped issue files are triaged and resolved correctly, `internal/core/sync_test.go` contains the required fixes only, `make verify` passes after the final code change, and exactly one local commit is created for this batch.

Constraints/Assumptions:

- Scoped code file: `internal/core/sync_test.go`.
- Scoped issue files only: `.compozy/tasks/reviews-watch/reviews-004/issue_001.md` and `issue_002.md`.
- Must not modify issue files outside this batch.
- Must not use destructive git commands.
- Workspace is already dirty in unrelated files; do not touch or revert them.
- `reviews-004/_meta.md` appears absent; rely on batch metadata plus issue files unless a local round meta file is found elsewhere.

Key decisions:

- Treat this as a review-remediation/test-hardening task, so root-cause analysis applies before edits.
- Keep changes constrained to the scoped test file and the two scoped issue markdown files.
- Verify narrowly while iterating, then run full `make verify` before finalizing issue status or committing.

State:

- In progress.

Done:

- Read repository instructions from `AGENTS.md` and `CLAUDE.md`.
- Loaded required skills: `cy-fix-reviews`, `cy-final-verify`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `golang-pro`.
- Scanned existing ledger files for cross-agent awareness.
- Read both scoped issue files.
- Read relevant cross-agent ledgers: `2026-04-30-MEMORY-reviews-watch.md` and `2026-04-30-MEMORY-review-watch-task-02.md`.
- Inspected current dirty worktree with `git status --short`.
- Began inspecting `internal/core/sync_test.go`.
- Confirmed `reviews-004` contains only the two scoped issue files; no round `_meta.md` exists in that directory.
- Triaged both scoped issues as `valid` with concrete remediation notes.
- Patched `internal/core/sync_test.go` to add the missing `review_issues` prune assertion and wrap the five cited single-scenario tests in `t.Run("Should ...")` subtests.
- Ran `gofmt -w internal/core/sync_test.go`.
- Ran focused verification: `go test ./internal/core` passed.

Now:

- Run full repository verification with `make verify`, then resolve the issue files and create the required local commit if the gate is clean.

Next:

- Update issue markdown frontmatter to `resolved` after clean verification.
- Create one local commit for this batch.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-30-MEMORY-reviews-watch-round4.md`
- `.compozy/tasks/reviews-watch/reviews-004/issue_001.md`
- `.compozy/tasks/reviews-watch/reviews-004/issue_002.md`
- `internal/core/sync_test.go`
- `git status --short`
- `sed -n` reads for issue files, ledgers, and test file
- `gofmt -w internal/core/sync_test.go`
- `go test ./internal/core`
