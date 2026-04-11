---
status: pending
file: internal/cli/state.go
line: 360
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093865430,nitpick_hash:e74f19343f96
review_hash: e74f19343f96
source_review_id: "4093865430"
source_review_submitted_at: "2026-04-11T14:27:18Z"
---

# Issue 002: Consider using s.isInteractive() for testability consistency.
## Review Comment

Line 360 calls `isInteractiveTerminal()` directly, but elsewhere in the codebase (e.g., `maybeCollectInteractiveParams` at line 248-252), the code uses `s.isInteractive` which allows test injection. This inconsistency could make testing `normalizePresentationMode` harder in scenarios where you want to mock the interactive state.

## Triage

- Decision: `UNREVIEWED`
- Notes:
