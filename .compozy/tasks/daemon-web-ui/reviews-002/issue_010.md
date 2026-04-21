---
status: resolved
file: internal/daemon/query_helpers_test.go
line: 234
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:fad243f5ec6f
review_hash: fad243f5ec6f
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 010: Consider extracting subtests for each error branch.
## Review Comment

This test covers four distinct scenarios: missing document read (lines 243-251), nil daemon state (lines 253-259), status error propagation (lines 261-265), and health error propagation (lines 267-274), plus the no-reviews case (lines 276-289). Subtests would improve clarity.

As per coding guidelines: "MUST use t.Run("Should...") pattern for ALL test cases".

---

## Triage

- Decision: `valid`
- Notes:
  - `TestQueryServiceReadHelpersHandleOptionalAndErrorBranches` covers multiple independent branches in one flow and mutates shared service state across checks.
  - Implemented: restructured the helper coverage into explicit `Should...` subtests so the missing-document, nil-daemon, error-propagation, and no-reviews branches are isolated and easier to diagnose.
  - Verification: `go test ./internal/daemon -run 'Test(HostRuntimeBehaviors|QueryHelperErrorsAndDocumentTitles|QueryHelperDirectoryAndStatusBranches|QueryServiceReadHelpersHandleOptionalAndErrorBranches)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
