Goal (incl. success criteria):

- Implement the accepted plan for `fetch-reviews` so CodeRabbit review-body parsing captures real `minor` and `major` comments, and simplify configuration by removing the CLI/TUI `nitpicks` toggle while keeping `.compozy/config.toml` as the only override.
- Success requires regression coverage for the real parser shape, updated CLI/config/docs behavior, and a clean `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills active: `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `cy-final-verify`.
- Accepted plan is persisted under `.codex/plans/2026-04-14-fetch-reviews-review-body-default.md`.
- Do not rename internal `Nitpicks` / `IncludeNitpicks` contracts or SDK wire fields in this task.
- If `.compozy/config.toml` omits `[fetch_reviews].nitpicks`, default behavior must be enabled.

Key decisions:

- Fix the parser at the root cause by supporting the real CodeRabbit review-body format instead of weakening severity/category detection.
- Keep config key `fetch_reviews.nitpicks`; remove only the CLI flag and form toggle.
- Apply config override without depending on a corresponding Cobra flag, because the flag is being removed.
- Preserve review-body comment severity mapping as `nitpick`, `minor`, and `major`.

State:

- Completed after fresh full verification.

Done:

- Read workspace instructions and related ledgers.
- Read required skill guidance for Go, debugging, no-workarounds, test changes, and final verification.
- Confirmed current worktree is clean before edits.
- Confirmed the likely parser gap: `parseReviewBodyCommentsForFile` only matches the simplified inline-title form and misses the real CodeRabbit metadata-plus-title structure.
- Confirmed an additional real-markup gap: file-level `<summary>` values can include a trailing `-<lineRange>` suffix such as `message-bubble.tsx-5-5`, which needed normalization back to the true file path.
- Confirmed current CLI/config plumbing still depends on the public `--nitpicks` flag and TUI field.
- Persisted the accepted plan and created this session ledger.
- Updated the CodeRabbit parser to support both the legacy inline-title form and the real metadata-plus-title form, while keeping review-body severity/hash semantics stable.
- Normalized review-body file summaries that append `-<lineRange>` to the path when that suffix mirrors the parsed comment location.
- Removed the public `--nitpicks` flag and the fetch-reviews TUI toggle.
- Made fetch-reviews default to review-body comments enabled in `commandState`, while keeping `[fetch_reviews].nitpicks` as the only config override.
- Updated CLI/help/config documentation and focused tests to reflect the new CLI/config behavior.
- Focused verification passed:
  - `go test ./internal/core/provider/coderabbit ./internal/cli -count=1`
  - `go test ./internal/core ./internal/core/workspace ./internal/core/kernel/commands ./internal/core/provider -count=1`
- Full verification passed:
  - `make verify`
  - Result: `0 issues`, `DONE 1830 tests`, successful `go build ./cmd/compozy`

Now:

- Prepare final handoff with verification evidence.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/plans/2026-04-14-fetch-reviews-review-body-default.md`
- `.codex/ledger/2026-04-14-MEMORY-fetch-reviews-review-body.md`
- `internal/core/provider/coderabbit/{coderabbit.go,nitpicks.go,nitpicks_test.go}`
- `internal/cli/{commands.go,form.go,state.go,workspace_config.go,form_test.go,root_test.go,workspace_config_test.go}`
- `internal/core/{fetch.go,fetch_test.go}`
- `internal/core/workspace/{config_types.go,config_test.go}`
- `README.md`
- `skills/compozy/references/{cli-reference.md,config-reference.md}`
- Commands: `rg`, `sed`, `go test ...`, `make verify`
