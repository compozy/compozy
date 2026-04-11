---
status: resolved
file: internal/core/extension/enablement_test.go
line: 10
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:772a9db2005f
review_hash: 772a9db2005f
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 024: Add t.Parallel() to enable concurrent test execution.
## Review Comment

Top-level test functions are missing `t.Parallel()` calls. Since these tests use `t.TempDir()` for isolation, they can safely run in parallel.

## Triage

- Decision: `valid`
- Notes: Most top-level tests in `enablement_test.go` are isolated and can safely use `t.Parallel()`. One exception remains serial because it overrides the package-global `osUserHomeDir`, but the rest should run in parallel to follow repository guidance and reduce unnecessary serialization.
