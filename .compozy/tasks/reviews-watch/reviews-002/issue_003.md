---
provider: coderabbit
pr: "133"
round: 2
round_created_at: 2026-04-30T21:47:34.803875Z
status: pending
file: internal/core/reviews/store_test.go
line: 256
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208605896,nitpick_hash:fcc23af9dffb
review_hash: fcc23af9dffb
source_review_id: "4208605896"
source_review_submitted_at: "2026-04-30T21:19:28Z"
---

# Issue 003: Add a populated pr scenario to this table
## Review Comment

This table now validates only empty/missing `pr`. Add a case like `pr: "259"` with `wantPR: "259"` to guard against regressions where non-empty PR is dropped during refresh/persist.

As per coding guidelines, "MUST test meaningful business logic, not trivial operations."

## Triage

- Decision: `UNREVIEWED`
- Notes:
