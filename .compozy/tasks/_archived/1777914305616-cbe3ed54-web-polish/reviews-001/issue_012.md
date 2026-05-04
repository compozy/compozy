---
status: resolved
file: web/src/systems/spec/components/workflow-spec-view.test.tsx
line: 162
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:b1e3e4b84a58
review_hash: b1e3e4b84a58
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 012: Add a test for the “active tab disappears” fallback too.
## Review Comment

This covers the `workflow.slug` reset path, but the new validity guard in `workflow-spec-view.tsx` is a separate branch. Please also rerender the same workflow after removing the active document and assert that the view falls back to the first present tab; that is the easier regression to miss here.

## Triage

- Decision: `valid`
- Notes:
  - `workflow-spec-view.tsx` already contains a branch that falls back when the currently active tab disappears, but the test suite only covers the workflow-change reset path.
  - That leaves the "active tab removed from the same workflow" regression unprotected.
  - Fix: extend the existing spec view test to rerender after removing the active document and assert fallback to the first present tab.
