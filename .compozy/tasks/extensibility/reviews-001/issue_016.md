---
status: resolved
file: internal/core/extension/assets.go
line: 134
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:50df13f395b4
review_hash: 50df13f395b4
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 016: Silent glob error handling is acceptable but consider logging for observability.
## Review Comment

The `resolveSkillPattern` method silently returns `nil` when glob operations fail (lines 142-144, 154-156). While this is robust for production, consider adding debug-level logging for observability when glob patterns fail to match, which could help diagnose configuration issues.

## Triage

- Decision: `invalid`
- Notes: `resolveSkillPattern` is a pure asset-extraction helper with no logger or request context, so adding ad hoc logging here would couple a deterministic helper to global side effects. The reported `Glob` errors only arise from malformed patterns, and that kind of observability belongs in manifest validation/discovery rather than this low-level collector. No correctness bug is demonstrated in the current batch scope.
