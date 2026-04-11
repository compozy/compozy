---
status: resolved
file: internal/core/agent/session_helpers_test.go
line: 575
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:ef29c8c08fff
review_hash: ef29c8c08fff
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 015: Consider relocating this assertion to the subprocess package tests.
## Review Comment

This test now validates `subprocess.NormalizeWaitError` behavior, which is defined in a different package. Testing external package behavior here creates coupling and may result in duplicate test coverage. Consider whether this assertion belongs in `internal/core/subprocess` tests instead.

## Triage

- Decision: `INVALID`
- Notes:
  - This is an organizational nitpick, not a bug. The assertion is a small smoke check in the ACP helper test file and does not affect product behavior.
  - `internal/core/subprocess` already has direct `NormalizeWaitError` coverage, so moving this extra assertion would only reshuffle test ownership without fixing a correctness gap.
  - No production or regression issue remains to address for this batch.
