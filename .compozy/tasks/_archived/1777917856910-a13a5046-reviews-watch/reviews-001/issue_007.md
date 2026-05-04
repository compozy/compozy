---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/core/reviews/store.go
line: 211
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:dc699c720c2c
review_hash: dc699c720c2c
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 007: Wrap metadata extraction errors with call-site context.
## Review Comment

Returning raw `err` here reduces diagnosability for this boundary. Keep `%w` wrapping so upstream matching still works.

As per coding guidelines, "Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`".

## Triage

- Decision: `valid`
- Root cause: `SnapshotRoundMeta` returns the raw metadata extraction error from `roundMetaFromIssueFrontMatter`, which drops the `SnapshotRoundMeta` call-site context and makes diagnostics less specific.
- Fix plan: Wrap the error with `SnapshotRoundMeta` context and add a focused regression test outside the listed scope because the existing round-meta tests live in `internal/core/reviews/store_test.go`.
