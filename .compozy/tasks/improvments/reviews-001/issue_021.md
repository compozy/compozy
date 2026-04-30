---
status: resolved
file: internal/core/migration/migrate.go
line: 428
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:ecd36e0704a3
review_hash: ecd36e0704a3
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 021: Missing PR metadata is skipped here.
## Review Comment

A frontmatter review file with `Provider`, `Round`, and `RoundCreatedAt` populated but an empty `PR` will be treated as complete and won't be rehydrated from the legacy `_meta.md`, leaving it partially migrated.

## Triage

- Decision: `valid`
- Notes: Confirmed `reviewFileNeedsRoundMetadata` treated provider/round/created_at as complete even when `pr` was empty. The migration path could skip legacy `_meta.md` rehydration and leave a partially migrated issue file. Fixed by considering empty `PR` as missing round metadata for migration formatting.
