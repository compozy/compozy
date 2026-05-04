---
status: resolved
file: internal/api/httpapi/static.go
line: 122
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:a812f9e86e8d
review_hash: a812f9e86e8d
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 002: Optional optimization: prevent duplicate hashing on concurrent cache misses.
## Review Comment

Two simultaneous misses for the same asset can both compute SHA-256 before either writes the cache entry. A second check under write lock avoids redundant work.

## Triage

- Decision: `invalid`
- Notes:
  - The current `etagFor` implementation is race-safe: concurrent misses may hash the same bytes twice, but they deterministically compute the same ETag and store the same value.
  - This is an optional micro-optimization, not a correctness defect or user-visible regression, and the review does not provide benchmark evidence that it matters on this path.
  - Skipping the change keeps the batch focused on real behavioral and accessibility issues instead of speculative optimization churn.
