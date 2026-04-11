---
status: resolved
file: internal/core/extension/chain.go
line: 164
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:2a6066026031
review_hash: 2a6066026031
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 019: WantsEvent returns true for empty filter - verify this is intentional.
## Review Comment

When `e.eventKinds` is empty (line 172-174), the method returns `true`, meaning the extension wants all events. This is likely intentional (opt-out filtering), but should be documented clearly.

## Triage

- Decision: `valid`
- Notes: The empty-filter behavior is intentional: no server-side event filter means the extension receives all events. The current implementation is correct, but the method comment should state that contract explicitly so readers do not misread the default as an oversight.
