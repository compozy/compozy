---
status: resolved
file: internal/daemon/query_helpers_test.go
line: 97
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:fa6bb98061b3
review_hash: fa6bb98061b3
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 009: Consider splitting into subtests for directory reading, lane titles, and job counts.
## Review Comment

This function tests several distinct behaviors: markdown directory walking (lines 114-130), lane/title normalization (lines 132-140), and job count aggregation (lines 142-155). Subtests would clarify what failed and allow parallel execution of independent cases.

As per coding guidelines: "MUST use t.Run("Should...") pattern for ALL test cases".

---

## Triage

- Decision: `valid`
- Notes:
  - `TestQueryHelperDirectoryAndStatusBranches` currently bundles directory walking, title normalization, and run-job count aggregation into one flat function.
  - Implemented: split those branches into focused `Should...` subtests so failures identify the broken helper immediately.
  - Verification: `go test ./internal/daemon -run 'Test(HostRuntimeBehaviors|QueryHelperErrorsAndDocumentTitles|QueryHelperDirectoryAndStatusBranches|QueryServiceReadHelpersHandleOptionalAndErrorBranches)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
