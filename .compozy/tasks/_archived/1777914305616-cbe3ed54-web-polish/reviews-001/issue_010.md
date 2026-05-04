---
status: resolved
file: web/src/systems/app-shell/components/workspace-onboarding.tsx
line: 50
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:1c7d2c6cf28b
review_hash: 1c7d2c6cf28b
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 010: Use unique IDs for assistive-text bindings
## Review Comment

Line 50 and Line 51 use hard-coded IDs, which can collide if this component is rendered multiple times on one page. Prefer `useId()` so `aria-describedby` targets stay unique.

## Triage

- Decision: `valid`
- Notes:
  - `WorkspaceOnboarding` hard-codes `aria-describedby` target IDs, so rendering multiple instances on one page creates duplicate IDs and ambiguous assistive-text bindings.
  - The defect is in the component's ID generation strategy, not in consumer usage.
  - Fix: switch to `useId()`-derived IDs and add a focused component test that renders two instances and verifies unique bindings.
