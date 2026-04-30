---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: sdk/extension/internal_test.go
line: 241
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:24a20d86c3b0
review_hash: 24a20d86c3b0
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 028: Use Should... subtest names to match required test style.
## Review Comment

The new subtests use raw hook names; please rename them to the `Should...` convention required for test cases.

As per coding guidelines: `MUST use t.Run("Should...") pattern for ALL test cases`.

## Triage

- Decision: `valid`
- Notes: The hook classification loop uses raw hook-name strings as subtest names, which violates the repository’s `Should...` test-case naming rule. I will rename those subtests while preserving the current mutability assertions.
- Resolution: Renamed the review-watch hook mutability subtests to the required `Should...` form and kept the underlying assertions intact; verified via `make verify`.
