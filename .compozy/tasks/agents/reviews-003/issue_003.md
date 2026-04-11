---
status: resolved
file: internal/core/run/exec/exec_test.go
line: 459
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4092869982,nitpick_hash:aa91bcee7adf
review_hash: aa91bcee7adf
source_review_id: "4092869982"
source_review_submitted_at: "2026-04-10T23:18:05Z"
---

# Issue 003: Avoid panic in this test helper; use t.Helper() and fail through testing.T.
## Review Comment

Line 465 panics on marshal failure, which gives poorer failure attribution than a helper-aware test failure.

As per coding guidelines: `Mark test helper functions with t.Helper() so stack traces point to the caller`.

## Triage

- Decision: `INVALID`
- Notes:
  - The observation is correct in isolation, but it is fully subsumed by `issue_002`, which already covers the same helper (`preparedPromptTextContentBlock`) and the need to replace the panic with helper-aware test failure handling.
  - Treating this as a separate valid item would duplicate the same code change and verification work.
