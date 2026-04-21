---
status: resolved
file: internal/daemon/host_runtime_test.go
line: 20
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:3f59f5cf33f3
review_hash: 3f59f5cf33f3
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 007: Adopt t.Run("Should...") subtests for these test cases.
## Review Comment

These tests are currently single-case top-level functions; the repository test standard requires `t.Run("Should...")` as the default structure.

As per coding guidelines, `**/*_test.go`: "Table-driven tests with subtests (`t.Run`) as the default pattern" and "MUST use `t.Run("Should...")` pattern for ALL test cases".

## Triage

- Decision: `valid`
- Notes:
  - The file currently mixes multiple single-case top-level tests without `Should...` subtest names, which makes failures less local and less consistent with the surrounding daemon test style.
  - Implemented: wrapped the host runtime cases in named `Should...` subtests while preserving the original behavioral coverage.
  - Verification: `go test ./internal/daemon -run 'Test(HostRuntimeBehaviors|QueryHelperErrorsAndDocumentTitles|QueryHelperDirectoryAndStatusBranches|QueryServiceReadHelpersHandleOptionalAndErrorBranches)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
