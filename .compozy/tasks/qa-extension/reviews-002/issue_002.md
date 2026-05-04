---
provider: coderabbit
pr: "138"
round: 2
round_created_at: 2026-05-02T04:56:54.019903Z
status: pending
file: extensions/cy-qa-workflow/main.go
line: 347
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4214432111,nitpick_hash:367ee2cda81a
review_hash: 367ee2cda81a
source_review_id: "4214432111"
source_review_submitted_at: "2026-05-02T04:41:56Z"
---

# Issue 002: Runtime task detection lacks body marker check, unlike task detection.
## Review Comment

`isQAReportRuntimeTask` and `isQAExecutionRuntimeTask` rely solely on title/type heuristics, while `isQAReportTask` and `isQAExecutionTask` also check for body markers. If `TaskRuntimeTask` has a `Body` field available, consider adding marker checks for consistency. If `Body` isn't available in that type, the current approach is acceptable.

## Triage

- Decision: `UNREVIEWED`
- Notes:
