---
provider: coderabbit
pr: "131"
round: 2
round_created_at: 2026-04-30T16:05:39.30025Z
status: resolved
file: packages/ui/src/components/logo.tsx
line: 66
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4206542727,nitpick_hash:dcad6c7e2cbe
review_hash: dcad6c7e2cbe
source_review_id: "4206542727"
source_review_submitted_at: "2026-04-30T15:47:24Z"
---

# Issue 004: Consider deriving SVG dimensions from a single source of truth.
## Review Comment

The `svgSizes` object duplicates the width/height values already defined in `logoVariants.svg`. If either is updated, they could drift out of sync.

One approach is to define the sizes once and derive both the Tailwind classes and the numeric values from that source. That said, this is low-risk since the logo dimensions rarely change.

## Triage

- Decision: `VALID`
- Notes:
  - `logoVariants` stores the SVG size classes while `svgSizes` repeats the matching numeric dimensions in the component body.
  - The duplication can drift if a logo size changes, and the numeric size map is recreated on every render.
  - Fix approach: define one module-level size table and derive both the Tailwind SVG classes and numeric SVG dimensions from it.
  - Resolution: implemented and verified with the repository verification pipeline.
