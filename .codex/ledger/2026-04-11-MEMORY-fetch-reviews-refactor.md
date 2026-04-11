Goal (incl. success criteria):

- Implement the accepted plan for three scoped changes:
  - make `fetch-reviews` use the same workflow-directory selector pattern as other interactive CLI forms
  - import CodeRabbit review-body comments for `nitpick`, `minor`, and `major`
  - remove the root `command/` package and move the public reusable Cobra API to package `compozy`
- Success requires targeted regression coverage and a clean `make verify`.

Constraints/Assumptions:

- Follow repository `AGENTS.md` and `CLAUDE.md`.
- Required skills loaded for this session: `golang-pro`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `cy-final-verify`, plus the user-requested `refactoring-analysis` and `architectural-analysis` context.
- Accepted plan was persisted under `.codex/plans/2026-04-11-fetch-reviews-cli-coderabbit-root-refactor.md`.
- Public API compatibility decision is locked: this is a breaking change; do not preserve the old `github.com/compozy/compozy/command` import path.
- Workspace is already dirty in unrelated files (`agents/`, `internal/setup/*`, task review files, another ledger); do not touch or revert them.

Key decisions:

- Keep the CLI flag/config key name `nitpicks` for now, but broaden its behavior to include CodeRabbit review-body categories `nitpick`, `minor`, and `major`.
- Preserve CodeRabbit category as normalized `severity` instead of mapping to generic levels.
- Treat review-body comments as one family in the fetch-history pipeline; rename nitpick-specific internals to neutral review-body comment terminology.
- Move public Cobra entry helpers into package `compozy` as `NewCommand` and `ExitCode`, then remove the root `command/` package.

State:

- Completed after fresh focused verification and a clean `make verify`.

Done:

- Explored the relevant CLI, provider, public API, README, and tests.
- Confirmed the current `fetch-reviews` form is intentionally blocked from using a directory selector by existing tests.
- Confirmed the current CodeRabbit parser only recognizes `nitpick comments` top-level details blocks.
- Confirmed the repo exposes `command.New()` publicly in `cmd/compozy`, README, and `test/public_api_test.go`.
- Persisted the accepted plan and created this session ledger.
- Refactored `internal/cli/form.go` so workflow-name selection is chosen by `commandKind`, enabling `fetch-reviews` to use a directory `Select` when workflows exist and preserving input fallback when none exist.
- Updated CLI/help copy so the existing `nitpicks` toggle now explicitly covers CodeRabbit review-body `nitpick`, `minor`, and `major` comments.
- Generalized the CodeRabbit review-body parser to scan all matching top-level details blocks, preserve category-specific severities, and keep deduplication/history tracking category-agnostic by hash.
- Renamed the fetch-history internals from nitpick-specific naming to neutral review-body comment naming and added regression coverage for newer reviews with changed severity.
- Moved the public reusable Cobra API into package `compozy` as `NewCommand()` and `ExitCode()`, updated `cmd/compozy`, README, and public API tests, and removed the root `command/` package.
- Focused verification passed:
  - `go test ./internal/cli ./internal/core/provider/coderabbit ./internal/core -count=1`
  - `go test ./cmd/compozy ./internal/cli ./internal/core/provider/coderabbit ./internal/core ./test -count=1`
- Full verification passed:
  - `make verify`
  - Result: `0 issues`, `DONE 1672 tests`, successful `go build ./cmd/compozy`

Now:

- Prepare the final handoff with the breaking public API note and fresh verification evidence.

Next:

- Optional cleanup only; no implementation work remains.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED whether any external documentation besides README references the old `command/` import path; update anything local that does.

Working set (files/ids/commands):

- `.codex/plans/2026-04-11-fetch-reviews-cli-coderabbit-root-refactor.md`
- `.codex/ledger/2026-04-11-MEMORY-fetch-reviews-refactor.md`
- `internal/cli/{form.go,form_test.go,commands.go,workspace_config.go}`
- `internal/core/provider/coderabbit/{coderabbit.go,nitpicks.go,nitpicks_test.go,coderabbit_test.go}`
- `internal/core/{fetch.go,fetch_test.go}`
- `compozy.go`
- `cmd/compozy/main.go`
- `test/public_api_test.go`
- `README.md`
- Commands: `rg`, `sed`, `go test ...`, `make verify`
