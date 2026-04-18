---
status: resolved
file: internal/api/core/sse_test.go
line: 178
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134973697,nitpick_hash:54c39d0af166
review_hash: 54c39d0af166
source_review_id: "4134973697"
source_review_submitted_at: "2026-04-18T19:43:56Z"
---

# Issue 006: Use a sentinel write error instead of matching error text.
## Review Comment

This assertion is brittle because it depends on the exact wrapped message. Return a stable sentinel from `failingFlushWriter.Write` and assert it with `errors.Is`, so the test verifies wrapping without coupling to the current string.

As per coding guidelines, "Use errors.Is() and errors.As() for error matching; do not compare error strings" and "MUST have specific error assertions (ErrorContains, ErrorAs)".

## Triage

- Decision: `valid`
- Root cause: the write-failure SSE test matches wrapped error text instead of a stable error value, which makes the assertion brittle against harmless message refactors.
- Fix plan: return a sentinel write error from the failing writer and assert wrapping with `errors.Is`.
- Resolution: `internal/api/core/sse_test.go` now uses a sentinel write error plus `errors.Is`, and it also covers `io.ErrShortWrite` so SSE write wrapping is validated without string matching.
