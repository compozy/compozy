---
status: resolved
file: internal/core/extension/host_writes.go
line: 397
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:80105e892f04
review_hash: 80105e892f04
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 029: Atomic write implementation is correct but could use fsync for durability.
## Review Comment

The temp-file-then-rename pattern is correct for atomicity. However, on some filesystems, the content may not be durable without an explicit `fsync` before closing. Consider whether durability guarantees are needed for this use case.

## Triage

- Decision: `valid`
- Notes: The temp-file-then-rename flow is already atomic, but the helper does not `Sync` the temporary file before closing and renaming it. Since this path writes host-side task/artifact content that should survive crashes more reliably, adding the file sync is a justified durability improvement while the helper is already being updated for the scoped write-path fixes.
