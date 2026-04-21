---
status: resolved
file: internal/api/core/handlers_contract_test.go
line: 59
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149167497,nitpick_hash:64b31ff82707
review_hash: 64b31ff82707
source_review_id: "4149167497"
source_review_submitted_at: "2026-04-21T16:03:49Z"
---

# Issue 001: Consider using "Should..." prefix in subtest names for consistency.
## Review Comment

Per coding guidelines, test cases should use the `t.Run("Should...")` pattern. For example: `"Should return degraded status"` instead of `"degraded"`.

---

## Triage

- Decision: `INVALID`
- Reasoning: The file already uses table-driven subtests correctly, and the documented repo guidance requires subtests by default, not a mandatory `Should...` prefix for every subtest label.
- Root cause: This is a naming preference, not a correctness, reliability, or coverage defect.
- Resolution plan: No code change. Close the issue as a cosmetic suggestion that is not required for this batch.

## Resolution

- Closed as `invalid`. The existing concise case names remain unchanged because there is no repository rule requiring a `Should...` prefix for these subtests.

## Verification

- Confirmed against the current test file and completed a fresh `make verify` pass after the in-scope fixes for the valid issues.
