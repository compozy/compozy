---
provider: coderabbit
pr: "133"
round: 2
round_created_at: 2026-04-30T21:47:34.803875Z
status: resolved
file: internal/core/agent/client_test.go
line: 67
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208605896,nitpick_hash:8908165ba093
review_hash: 8908165ba093
source_review_id: "4208605896"
source_review_submitted_at: "2026-04-30T21:19:28Z"
---

# Issue 002: Ensure client teardown always runs, even on early assertion failures.
## Review Comment

If an assertion fails before Line 86, `client.Close()` is skipped and the helper process can leak into other parallel tests. Register cleanup right after client creation.

## Triage

- Decision: `UNREVIEWED`
- Notes:
