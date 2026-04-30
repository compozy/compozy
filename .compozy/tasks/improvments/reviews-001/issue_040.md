---
status: resolved
file: internal/store/globaldb/read_queries.go
line: 121
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:7b584a128883
review_hash: 7b584a128883
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 040: Add a defensive fallback when blob body rows are missing.
## Review Comment

If `body_storage_kind` is blob but `artifact_bodies` has no matching checksum row, this branch returns `NULL` and callers get an empty body. Consider `COALESCE` to prevent silent data loss in degraded states.

## Triage

- Decision: `valid`
- Notes: Confirmed blob-backed snapshot reads returned `NULL` body text when the matching `artifact_bodies` row was absent. Added `COALESCE(bodies.body_text, snapshots.body_text)` so degraded rows fall back to the inline snapshot body instead of silently returning empty content.
