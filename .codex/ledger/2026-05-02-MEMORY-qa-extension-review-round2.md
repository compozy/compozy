Goal (incl. success criteria):

- Remediate CodeRabbit batch `qa-extension` PR 138 review round 002 across issue files `issue_001.md` through `issue_007.md`.
- Success = every scoped issue file is read and triaged, valid items are fixed with tests as needed, issue files move to `resolved` only after verification, `make verify` passes, and exactly one local commit is created.

Constraints/Assumptions:

- Use `cy-fix-reviews` as the workflow source of truth and `cy-final-verify` before any completion claim or commit.
- Only the scoped issue files may be updated for review bookkeeping.
- Code-change scope is limited to:
  - `.codex/tmp/qa-workflow-extension-lab/.compozy/extensions/cy-qa-workflow/main.go`
  - `extensions/cy-qa-workflow/main.go`
  - `extensions/cy-qa-workflow/main_test.go`
  - `internal/core/extension/host_writes.go`
  - `sdk/extension/extension_test.go`
  - `sdk/extension/types.go`
  - `sdk/extension/types_test.go`
- Do not call provider-specific scripts, `gh` mutations, or destructive git commands.
- Review directory `_meta.md` is absent; use the user-provided batch scope plus the issue files as the available round context.

Key decisions:

- Actual code scope is in `/Users/pedronauck/Dev/compozy/looper`, not the current `agh` repo; follow looper-local instructions for code changes there.
- Reuse the existing `2026-05-02-MEMORY-prompt-goal-extension.md` ledger only as historical context; keep this remediation in its own session ledger.

State:

- In progress.

Done:

- Loaded required skills: `cy-fix-reviews`, `cy-final-verify`, `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`.
- Read looper workspace guidance from `AGENTS.md` / `CLAUDE.md`.
- Scanned existing ledgers and read the related `2026-05-02-MEMORY-prompt-goal-extension.md`.
- Read all seven scoped issue files completely.
- Confirmed the looper worktree only has untracked `.compozy/tasks/qa-extension/reviews-002/`.
- Inspected current implementations in the scoped Go source/test files.

Now:

- Triaging each issue against the current looper code and preparing the corresponding issue-file updates.

Next:

- Update all seven issue files from `pending` to `valid` or `invalid` with concrete reasoning.
- Implement code/test changes for valid items.
- Run verification, resolve issue files, and create one local commit.

Open questions (UNCONFIRMED if needed):

- Whether the temp-lab extension copy should stay synchronized with the canonical extension or whether the review item is invalid because the file is intentional QA-fixture state.

Working set (files/ids/commands):

- `.compozy/tasks/qa-extension/reviews-002/issue_001.md` through `issue_007.md`
- `extensions/cy-qa-workflow/main.go`
- `extensions/cy-qa-workflow/main_test.go`
- `internal/core/extension/host_writes.go`
- `sdk/extension/extension_test.go`
- `sdk/extension/types.go`
- `sdk/extension/types_test.go`
- `.codex/tmp/qa-workflow-extension-lab/.compozy/extensions/cy-qa-workflow/main.go`
- `.codex/ledger/2026-05-02-MEMORY-prompt-goal-extension.md`
