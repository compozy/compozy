---
status: resolved
file: internal/daemon/query_helpers_test.go
line: 36
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:ef754c8f2ad3
review_hash: ef754c8f2ad3
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 008: Consider restructuring with t.Run("Should...") subtests for better isolation and readability.
## Review Comment

This test function covers multiple distinct behaviors (three error types and five document title cases) in a flat structure. Breaking these into subtests would improve test isolation and make failures easier to diagnose.

Example structure:
```go
t.Run("Should match DocumentMissingError against ErrDocumentMissing", func(t *testing.T) { ... })
t.Run("Should extract title from task body", func(t *testing.T) { ... })
```

As per coding guidelines: "MUST use t.Run("Should...") pattern for ALL test cases".

## Triage

- Decision: `valid`
- Notes:
  - `TestQueryHelperErrorsAndDocumentTitles` currently checks several unrelated error/value branches in one flat block, so a single failure gives poor locality.
  - Implemented: split the error cases and title derivation cases into named `Should...` subtests without changing behavior.
  - Verification: `go test ./internal/daemon -run 'Test(HostRuntimeBehaviors|QueryHelperErrorsAndDocumentTitles|QueryHelperDirectoryAndStatusBranches|QueryServiceReadHelpersHandleOptionalAndErrorBranches)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
