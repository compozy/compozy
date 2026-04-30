---
status: resolved
file: internal/core/reviews/store_test.go
line: 117
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:9958e958b2b3
review_hash: 9958e958b2b3
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 023: Use errors.Is(err, fs.ErrNotExist) for these absence checks.
## Review Comment

These assertions introduce `os.IsNotExist`, but the repo standard is `errors.Is` / `errors.As` for error matching. Switching the not-found checks keeps the tests aligned with the rest of the Go code.

As per coding guidelines, "Use `errors.Is()` and `errors.As()` for error matching; do not compare error strings".

Also applies to: 246-247

## Triage

- Decision: `valid`
- Notes: Confirmed the tests used `os.IsNotExist` for `_meta.md` absence checks. Replaced those assertions with `errors.Is(err, fs.ErrNotExist)` to match the repository error-matching standard.
