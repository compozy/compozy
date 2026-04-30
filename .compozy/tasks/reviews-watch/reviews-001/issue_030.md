---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: web/src/systems/reviews/components/review-round-detail-view.test.tsx
line: 99
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4208391746,nitpick_hash:5fe9903d8c8a
review_hash: 5fe9903d8c8a
source_review_id: "4208391746"
source_review_submitted_at: "2026-04-30T20:37:25Z"
---

# Issue 030: Reuse one render tree for the state transitions.
## Review Comment

Each `renderRoundDetail(...)` call mounts a fresh router without unmounting the previous one, so the later assertions run against cumulative DOM. That makes this test brittle to duplicate test ids and leaked router subscriptions. Prefer `rerender(...)` or `cleanup()` between scenarios.

## Triage

- Decision: `valid`
- Notes: The state-transition test mounts the component three times without reusing or cleaning up the prior tree, so later assertions observe cumulative DOM and router instances. I will switch that test to a single render tree with rerendered props.
- Resolution: Switched the state-transition coverage to a single context-backed render tree with rerenders so the DOM no longer accumulates between scenarios; reverified with `make verify`.
