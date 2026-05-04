---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: web/src/systems/runs/components/runs-list-view.test.tsx
line: 141
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:afe9b0877d10
review_hash: afe9b0877d10
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 032: Add an assertion for the new status-badge non-shrinking behavior.
## Review Comment

This test already checks truncation and status text; asserting `shrink-0` on the badge would directly guard the layout regression fix from this PR.

## Triage

- Decision: `valid`
- Notes: The layout change that pins the run-status badge with `shrink-0` is not directly asserted in the test, so the review is correct that the regression coverage is incomplete. I will add an explicit class assertion to the long-identifier case.
- Resolution: Added the explicit `shrink-0` assertion to the runs-list regression test and reverified it in `make verify`.
