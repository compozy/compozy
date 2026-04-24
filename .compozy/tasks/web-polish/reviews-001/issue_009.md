---
status: resolved
file: packages/ui/tests/primitives.test.tsx
line: 153
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4167198630,nitpick_hash:5555573a615e
review_hash: 5555573a615e
source_review_id: "4167198630"
source_review_submitted_at: "2026-04-24T01:14:40Z"
---

# Issue 009: Tighten the disabled assertion
## Review Comment

Line 157 checks a broad `"disabled"` substring. A more specific button-attribute assertion makes this test less brittle.

## Triage

- Decision: `valid`
- Notes:
  - The current assertion checks for the substring `"disabled"` in serialized HTML, which is broader than the actual contract and could pass for unrelated markup changes.
  - The underlying behavior being protected is the button's `disabled` attribute when `loading` is true.
  - Fix: tighten the test to assert the concrete button attribute so the regression signal stays specific.
