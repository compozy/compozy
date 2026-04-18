---
status: resolved
file: internal/api/client/reviews_exec_test.go
line: 217
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134921970,nitpick_hash:bf8a82089826
review_hash: bf8a82089826
source_review_id: "4134921970"
source_review_submitted_at: "2026-04-18T18:54:28Z"
---

# Issue 003: Avoid string-matching the guard errors.
## Review Comment

These assertions are tied to the exact wording of the error, so a harmless message rewrite will fail the tests even if the behavior is still correct. Exposing sentinel or typed validation errors here would let the tests assert with `errors.Is`/`errors.As` instead.

As per coding guidelines, "Use `errors.Is()` and `errors.As()` for error matching; do not compare error strings" and "`*_test.go`: MUST have specific error assertions (ErrorContains, ErrorAs)".

## Triage

- Decision: `VALID`
- Root cause: the guard-error assertions in `reviews_exec_test.go` match raw error text, which makes the tests brittle and conflicts with the repo rule to use `errors.Is` / `errors.As`.
- Fix plan: expose sentinel client validation errors for the nil-client / blank-slug guards used here, switch the tests to `errors.Is`, and keep the public behavior otherwise unchanged. This requires a small production edit in `internal/api/client/reviews_exec.go` outside the listed code-file set because the guarded methods under test live there.
- Resolution: Implemented and verified with `make verify`.
