---
status: resolved
file: internal/core/extension/manager_test.go
line: 83
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:b23613ccc9a0
review_hash: b23613ccc9a0
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 018: Use command constants in assertions/fixtures to prevent drift.
## Review Comment

These updated checks use raw strings for command identity. Prefer `invokingCommandTasksRun` / `invokingCommandFixReviews` to keep tests aligned with constants.

Also applies to: 491-493, 834-838

## Triage

- Decision: `VALID`
- Notes: Several extension manager tests asserted raw command strings. Replaced the relevant fixtures/assertions with `invokingCommandTasksRun` and `invokingCommandFixReviews` to avoid drift from the canonical constants.
