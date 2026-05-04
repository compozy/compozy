---
status: resolved
file: internal/api/client/client.go
line: 192
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134973697,nitpick_hash:537ebf73b763
review_hash: 537ebf73b763
source_review_id: "4134973697"
source_review_submitted_at: "2026-04-18T19:43:56Z"
---

# Issue 003: Avoid context.Background() fallback in library code.
## Review Comment

Per coding guidelines, `context.Background()` should be avoided outside `main` and focused tests. Callers should be required to provide a valid context rather than silently substituting one.

## Triage

- Decision: `valid`
- Root cause: `(*Client).doJSON` silently replaces a nil caller context with `context.Background()`, which hides misuse inside reusable library code instead of forcing call sites to provide a real request context.
- Fix plan: reject nil contexts with a stable sentinel error and add focused client tests that assert `errors.Is(..., sentinel)` instead of permitting the fallback.
- Resolution: `internal/api/client/client.go` now returns `ErrDaemonContextRequired` for nil request contexts, and `internal/api/client/reviews_exec_test.go` covers the nil-context guard with `errors.Is`.
