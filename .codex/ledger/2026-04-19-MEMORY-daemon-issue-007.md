Goal (incl. success criteria):

- Resolve the scoped CodeRabbit review batch item `.compozy/tasks/daemon/reviews-003/issue_007.md` for PR `116`, round `003`.
- Success means: the finding is triaged against the current `internal/daemon/boot_test.go`, any required in-scope test fix is implemented, fresh verification evidence exists, and the issue artifact is closed as `resolved`.

Constraints/Assumptions:

- Follow `AGENTS.md`, `CLAUDE.md`, and the batch execution contract.
- Required skills read this session: `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`.
- Only `.compozy/tasks/daemon/reviews-003/issue_007.md` is in scope for review-artifact edits.
- Code-file scope is limited to `internal/daemon/boot_test.go`.
- The worktree is already dirty; do not overwrite or revert unrelated changes.
- Completion requires fresh full verification via `make verify`.

Key decisions:

- Treat the review comment as likely valid pending code edit because `internal/daemon/boot_test.go` still contains multiple `Host.Close(context.Background())` cleanup calls that discard returned errors.
- Constrain the code change to test cleanup only; do not alter production daemon behavior for this batch item.

State:

- In progress.

Done:

- Read the required skill guides for `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, and `no-workarounds`.
- Read `.compozy/tasks/daemon/reviews-003/_meta.md`.
- Read `.compozy/tasks/daemon/reviews-003/issue_007.md` completely before editing.
- Scanned daemon-related ledgers for cross-agent awareness.
- Inspected `internal/daemon/boot_test.go`, `internal/daemon/boot.go`, and `internal/daemon/host.go`.
- Confirmed the current worktree still contains ignored `Host.Close` errors at lines 49, 149, and 364 in `internal/daemon/boot_test.go`.

Now:

- Patch the ignored cleanup errors in `internal/daemon/boot_test.go`.

Next:

- Run focused daemon tests, then `make verify`.
- Update `.compozy/tasks/daemon/reviews-003/issue_007.md` with triage and verification evidence.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-19-MEMORY-daemon-issue-007.md`
- `.compozy/tasks/daemon/reviews-003/{_meta.md,issue_007.md}`
- `internal/daemon/{boot_test.go,boot.go,host.go}`
- `git status --short`
- `git diff -- internal/daemon/boot_test.go`
- `rg -n "_ = .*Close\\(|\\.Close\\(context\\.Background\\(\\)\\)" internal/daemon/boot_test.go internal/daemon/host.go internal/daemon/boot.go`
