---
status: resolved
file: packages/ui/src/components/metric.tsx
line: 5
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:d492f4f246fe
review_hash: d492f4f246fe
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 005: ReactNode slots don't match the <p> wrappers.
## Review Comment

`label`, `value`, and `hint` accept arbitrary JSX, but these slots are rendered inside `<p>` tags. Passing block content here will produce invalid nesting. Either narrow the props to text-like content or switch those wrappers to `<div>/<span>` so the public API matches what the component can safely render.

Also applies to: 30-35

## Triage

- Decision: `valid`
- Notes:
  - `MetricProps` accepts arbitrary `ReactNode` content, but `label`, `value`, and `hint` are rendered inside `<p>` tags, which allows invalid nesting when callers pass block content.
  - The mismatch is between the public API contract and the markup implementation.
  - Fix: switch those wrappers to non-paragraph containers that safely accept arbitrary React nodes and add a regression test with block content.
