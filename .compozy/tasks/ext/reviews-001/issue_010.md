---
status: resolved
file: internal/cli/run.go
line: 120
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:022e1fe43534
review_hash: 022e1fe43534
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 010: Consider using a pointer instead of variadic for optional single argument.
## Review Comment

The variadic signature `assets ...declarativeAssets` is unusual when only 0 or 1 asset is ever expected. A pointer `*declarativeAssets` would express intent more clearly.

## Triage

- Decision: `INVALID`
- Notes:
  - This is a style preference, not a correctness or maintainability bug in the current code.
  - `runPrepared` accepts either zero assets or one precomputed declarative asset bundle, and the variadic signature expresses that optional call-site shape without extra nil handling or a sentinel value.
  - Changing the signature to a pointer would be churn-only for this batch and does not address a demonstrated defect.
