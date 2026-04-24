---
status: resolved
file: web/src/systems/workflows/components/workflow-inventory-view.tsx
line: 234
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:0b88061c0d97
review_hash: 0b88061c0d97
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 013: Extract duplicated workflow action-link classes into one constant.
## Review Comment

Lines 234, 243, and 252 duplicate the same class string. Centralizing it will reduce drift and simplify future style changes.

## Triage

- Decision: `invalid`
- Notes:
  - The duplicated link class string is a maintainability nit, but it does not currently cause incorrect rendering or divergent behavior.
  - Extracting a local constant here would be a refactor-only change with no direct regression signal to validate in this batch.
  - Skipping it keeps the review-fix scope limited to actual bugs and accessibility/test gaps.
