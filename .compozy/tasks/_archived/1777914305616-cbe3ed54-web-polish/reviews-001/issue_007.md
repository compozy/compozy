---
status: resolved
file: packages/ui/src/components/status-badge.tsx
line: 68
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:9c06950c1fe4
review_hash: 9c06950c1fe4
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 007: Respect reduced-motion for the pulse affordance.
## Review Comment

The dot animation currently runs for everyone. In a shared status primitive, gate it with `motion-safe:` so users who prefer reduced motion still get the status color without the animation.

## Triage

- Decision: `valid`
- Notes:
  - `StatusBadge` always applies `animate-pulse` when `pulse` is true, ignoring users who prefer reduced motion.
  - This is an accessibility bug in a shared primitive because every consumer inherits the animation behavior.
  - Fix: gate the animation with `motion-safe:` while keeping the non-animated status indicator intact, and add a regression test on the rendered class list.
