---
status: resolved
file: internal/daemon/query_documents.go
line: 145
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:3fc8e96a8fa1
review_hash: 3fc8e96a8fa1
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 029: Normalize snapshot UpdatedAt to UTC for parity with file-backed documents.
## Review Comment

At Line 166, this path keeps original location while file-backed normalization uses UTC; this can create subtle ordering/serialization drift.

## Triage

- Decision: `valid`
- Notes: Confirmed snapshot-backed markdown documents copied `SourceMTime` without UTC normalization while file-backed documents use normalized times. Updated snapshot document construction to store `UpdatedAt` as UTC.
