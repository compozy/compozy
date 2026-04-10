Goal (incl. success criteria):

- Fix all review nitpicks that are still valid in the current tree, without applying workaround-style changes or broad speculative refactors.
- Success requires validating each nitpick against current code, changing only justified points, and finishing with clean `make verify`.

Constraints/Assumptions:

- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills loaded: `no-workarounds`, `golang-pro`, `systematic-debugging`, `testing-anti-patterns`; `cy-final-verify` before completion.
- Scope is limited to nitpick comments from the provided review dumps; non-nitpick comments are out of scope unless needed to support a valid nitpick fix.
- Do not touch unrelated files or use destructive git commands.

Key decisions:

- Treat each nitpick as a hypothesis, not an automatic fix.
- Reject nitpicks that are obsolete, redundant with current code, or would force speculative architecture churn beyond review scope.
- Reject the `internal/core/model/constants.go` nitpick as out-of-scope architectural churn for this review pass; it would require broader runtime-config plumbing rather than a contained nitpick fix.
- Keep `cloneMap` shallow and document the intent instead of deep-cloning nested structures, because current normalization treats nested values as read-only.

State:

- Completed with clean verification.

Done:

- Read workspace instructions and skill files.
- Scanned existing ledgers for cross-agent awareness.
- Confirmed clean worktree before changes.
- Validated nitpicks against current code and applied the contained fixes that still held:
  - bounded `waitForUpdateResult` plus direct tests in `cmd/compozy`
  - extracted shared `prepareAndRun` flow in `internal/cli/run.go`
  - made `withFallbacks` purely functional and documented defensive callback fallbacks
  - added journal handle interface assertion and wrapped close errors with context
  - propagated caller context into ACP start-command resolution
  - stopped swallowing `marshalRawJSON` errors and propagated them through callers
  - normalized repeated tool-name literals to constants, removed duplicate trim fallback, clarified precedence, and documented shallow clone intent
  - improved `failOnWriteWriter` fallback error
  - removed redundant `tt := tt` loop captures in the review-targeted tests
  - refactored targeted tests into `Should ...` subtests where requested
- Targeted verification passed:
  - `go test ./cmd/compozy ./internal/cli ./internal/core/agent ./internal/core/kernel ./internal/core/model -count=1`
- Full verification passed:
  - `make verify`
  - Result: `0 issues`, `DONE 1089 tests`, successful build of `bin/compozy`

Now:

- Prepare the final handoff with verified results.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-07-MEMORY-review-nitpicks.md`
- Review-targeted files under `cmd/compozy`, `internal/cli`, `internal/core/agent`, `internal/core/model`, `internal/core/plan`, `internal/core/kernel`
- Commands: `sed`, `rg`, `go test ./cmd/compozy ./internal/cli ./internal/core/agent ./internal/core/kernel ./internal/core/model -count=1`, `make verify`
