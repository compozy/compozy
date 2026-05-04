---
provider: coderabbit
pr: "138"
round: 2
round_created_at: 2026-05-02T04:56:54.019903Z
status: pending
file: sdk/extension/types.go
line: 373
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4214452326,nitpick_hash:8be192f2c3b3
review_hash: 8be192f2c3b3
source_review_id: "4214452326"
source_review_submitted_at: "2026-05-02T04:56:23Z"
---

# Issue 006: Wrap unmarshal errors with context
## Review Comment

Both `UnmarshalJSON` methods return raw decode errors; add contextual wrapping so callers can pinpoint which payload failed.

As per coding guidelines, `Prefer explicit error returns with wrapped context using fmt.Errorf("context: %w", err)`.

Also applies to: 409-411

## Triage

- Decision: `UNREVIEWED`
- Notes:
