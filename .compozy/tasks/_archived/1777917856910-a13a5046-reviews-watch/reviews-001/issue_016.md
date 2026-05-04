---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_watch_test.go
line: 629
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:d96307a99d6a
review_hash: d96307a99d6a
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 016: Mark independent failure-state subtests with t.Parallel().
## Review Comment

These table-driven subtests appear isolated (`newReviewWatchTestEnv` per case), so they should run in parallel for faster feedback and better race-safety coverage.

As per coding guidelines, "**Use t.Parallel() for independent subtests**".

## Triage

- Decision: `valid`
- Root cause: The failure-state table cases each construct isolated review-watch environments but do not mark the subtests parallel, missing the project’s preferred pattern for independent cases.
- Fix plan: Add `t.Parallel()` to the isolated failure-state subtests while preserving current case setup and assertions.
