---
status: resolved
file: internal/api/client/client_contract_test.go
line: 290
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:b5731fd75315
review_hash: b5731fd75315
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 003: Potential panic if response structure differs from expected.
## Review Comment

The assertion at line 290 accesses nested indices without verifying that `got.Messages` and `got.Messages[0].Parts` have the expected lengths. If the API response format changes or returns an empty/malformed response, this will panic instead of providing a clear test failure message.

Consider adding explicit length checks:

## Triage

- Decision: `VALID`
- Notes: The transcript contract test indexed `got.Messages[0].Parts[0]` after only checking the message count in a combined condition. Split the assertions so malformed response shape reports explicit length failures before indexing.
