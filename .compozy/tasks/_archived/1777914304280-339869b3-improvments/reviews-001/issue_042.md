---
status: resolved
file: internal/store/globaldb/sync_test.go
line: 349
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:1de3c14aea09
review_hash: 1de3c14aea09
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 042: Wrap the new oversized-body scenario in a t.Run("Should...") subtest.
## Review Comment

This case is substantial enough to benefit from the repository's default subtest structure, especially if more blob-storage scenarios get added here later.

As per coding guidelines, `**/*_test.go`: MUST use t.Run("Should...") pattern for ALL test cases.

## Triage

- Decision: `VALID`
- Notes: The oversized-body test was a standalone test body while nearby tests use subtests for substantial scenarios. The fix wraps the scenario in a `t.Run("Should...")` subtest while preserving `t.Parallel()`.
