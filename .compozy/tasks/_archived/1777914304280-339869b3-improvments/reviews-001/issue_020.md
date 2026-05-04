---
status: resolved
file: internal/core/fetch_test.go
line: 88
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:d1fbcd4e8e54
review_hash: d1fbcd4e8e54
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 020: Assert the frontmatter semantically, not by raw YAML formatting.
## Review Comment

These checks are tied to serializer details like quoting `pr`, so a harmless emission change can fail the test even when the stored metadata is still correct. Since this file already imports `reviews`, parsing the issue frontmatter and asserting `Provider`, `PR`, `Round`, and `RoundCreatedAt` would make the test much more stable.

## Triage

- Decision: `VALID`
- Notes: The fetch review test asserted raw YAML snippets, including serializer-sensitive quoting. It now parses the generated review issue frontmatter with `reviews.ParseReviewContext` and asserts provider, PR, round, and non-zero round creation time semantically.
