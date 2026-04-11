---
status: pending
file: internal/core/fetch.go
line: 166
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093865430,nitpick_hash:84789d006636
review_hash: 84789d006636
source_review_id: "4093865430"
source_review_submitted_at: "2026-04-11T14:27:18Z"
---

# Issue 004: Minor: Consider returning empty slice instead of nil.
## Review Comment

Returning `nil` when `len(items) == 0` is valid Go, but returning `[]provider.ReviewItem{}` would be more consistent with the non-empty case and avoids potential nil-slice vs empty-slice confusion for callers.

## Triage

- Decision: `UNREVIEWED`
- Notes:
