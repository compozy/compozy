---
status: pending
file: internal/core/provider/coderabbit/nitpicks_test.go
line: 85
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093865430,nitpick_hash:9e08b6208032
review_hash: 9e08b6208032
source_review_id: "4093865430"
source_review_submitted_at: "2026-04-11T14:27:18Z"
---

# Issue 006: Inconsistent terminology in error messages.
## Review Comment

The error messages at lines 85, 137, and 140 still reference "nitpick hash" while the rest of the codebase has been renamed to "review body comment". Consider updating for consistency.

Also applies to: 137-140

## Triage

- Decision: `UNREVIEWED`
- Notes:
