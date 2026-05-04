---
provider: coderabbit
pr: "138"
round: 2
round_created_at: 2026-05-02T04:56:54.019903Z
status: pending
file: internal/core/extension/host_writes.go
line: 479
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4214432111,nitpick_hash:93164972d626
review_hash: 93164972d626
source_review_id: "4214432111"
source_review_submitted_at: "2026-05-02T04:41:56Z"
---

# Issue 004: Note: taskIndexWorkflowTitle duplicates workflowTitle in the QA extension.
## Review Comment

Both functions have identical logic. Since they're in separate packages (internal vs extension), this may be intentional to avoid cross-package dependencies. If this pattern recurs, consider exposing a shared utility.

## Triage

- Decision: `UNREVIEWED`
- Notes:
