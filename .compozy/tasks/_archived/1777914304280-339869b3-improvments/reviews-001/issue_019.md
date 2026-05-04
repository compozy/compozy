---
status: resolved
file: internal/core/extension/review_provider_runtime_test.go
line: 28
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:a6e8314a2d2a
review_hash: a6e8314a2d2a
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 019: Prefer command constant over raw string in fixture.
## Review Comment

Line 30 hardcodes `"reviews fix"`. Using `invokingCommandFixReviews` avoids string drift when command labels change again.

## Triage

- Decision: `VALID`
- Notes: The review-provider runtime fixture hardcoded `reviews fix`. Replaced it with `invokingCommandFixReviews`.
