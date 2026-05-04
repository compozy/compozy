---
status: resolved
file: internal/api/core/sse_test.go
line: 37
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134921970,nitpick_hash:b5dffd33cc69
review_hash: b5dffd33cc69
source_review_id: "4134921970"
source_review_submitted_at: "2026-04-18T18:54:28Z"
---

# Issue 011: Missing t.Parallel() for test independence.
## Review Comment

This test function doesn't call `t.Parallel()`, unlike the other tests in this file. Adding it would allow this test to run concurrently with others.

As per coding guidelines: "Use `t.Parallel()` for independent subtests".

## Triage

- Decision: `VALID`
- Root cause: `TestWriteSSEFormatsFramesWithCanonicalCursor` is independent but does not opt into parallel execution like the neighboring tests in the same file.
- Fix plan: add `t.Parallel()` to the test.
- Resolution: Implemented and verified with `make verify`.
