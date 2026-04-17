---
status: resolved
file: internal/core/extension/review_provider_bridge.go
line: 187
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4094514795,nitpick_hash:3e4449afb8a7
review_hash: 3e4449afb8a7
source_review_id: "4094514795"
source_review_submitted_at: "2026-04-12T04:17:10Z"
---

# Issue 004: Consider consistent error wrapping when both shutdowns fail.
## Review Comment

When both the previous and new manager shutdowns fail (lines 194-199), the error wraps `cleanupErr` (new manager) with `%w` but formats `err` (previous manager) with `%v`. This means `errors.Unwrap` will return the new manager's error rather than the original failure that triggered cleanup. Both errors are present in the message which aids debugging, but consider whether the original error should be the wrapped cause for consistency with standard error chain conventions.

## Triage

- Decision: `invalid`
- Notes:
  - The current implementation already preserves both shutdown failures in the returned message, which is the only behavior consumed today.
  - Changing the wrapped cause would be a discretionary error-chain preference rather than a demonstrated bug fix. A stronger change here would likely be `errors.Join(...)`, which goes beyond the nitpick and would need dedicated tests outside this batch’s practical scope.
  - No code change is planned for this issue.
