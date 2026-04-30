---
status: resolved
file: internal/store/globaldb/migrations.go
line: 165
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:d1be76885c47
review_hash: d1be76885c47
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 039: Consider enforcing non-negative artifact_bodies.size_bytes at schema level.
## Review Comment

A DB check constraint prevents invalid size metadata from being persisted if upstream validation regresses.

## Triage

- Decision: `valid`
- Notes: Confirmed `artifact_bodies.size_bytes` had no schema-level non-negative guard. Added a `CHECK (size_bytes >= 0)` constraint to the artifact body table definition for new/migrated databases.
