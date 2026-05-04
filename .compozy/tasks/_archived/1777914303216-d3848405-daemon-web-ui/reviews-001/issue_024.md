---
status: resolved
file: packages/ui/src/components/section-heading.tsx
line: 5
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:93a061d9fb4d
review_hash: 93a061d9fb4d
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 024: Consider making heading level configurable for semantic flexibility.
## Review Comment

At Line 32, always rendering `h1` can produce invalid heading hierarchy in nested views. Adding an `as` prop improves reuse without changing default behavior.

## Triage

- Decision: `invalid`
- Notes:
  - This is a speculative API expansion rather than a concrete defect in the current tree. The current `SectionHeading` call sites are page and route heading blocks, and no in-scope usage reproduces a broken heading hierarchy.
  - Root cause of the comment: it anticipates a future reuse scenario, but there is no present failure or requirement that needs a configurable heading level in this batch.
  - Resolution path: do not widen the public component API without a concrete consumer need. If a nested-heading use case appears later, add the prop with the specific call site that requires it.

## Resolution

- Closed as invalid. No code change was made because the current tree does not reproduce a heading-hierarchy defect and adding a new public `as` prop would be speculative API growth.
- Verification:
- `make verify`
