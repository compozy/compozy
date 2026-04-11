---
status: resolved
file: internal/core/extension/assets_test.go
line: 8
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:d66fa87648e8
review_hash: d66fa87648e8
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 017: Consider adding t.Parallel() for independent tests.
## Review Comment

Both test functions appear to be independent and could benefit from parallel execution. As per coding guidelines, independent subtests should use `t.Parallel()`.

Also applies to: 51-51

## Triage

- Decision: `valid`
- Notes: Both tests in `assets_test.go` are isolated, use per-test temporary directories only, and do not mutate package-level globals. Adding `t.Parallel()` is safe here and matches the repository testing guidance.
