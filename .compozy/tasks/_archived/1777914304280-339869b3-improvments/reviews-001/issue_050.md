---
status: resolved
file: web/src/systems/workflows/components/workflow-inventory-view.test.tsx
line: 125
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:34447a3482f6
review_hash: 34447a3482f6
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 050: Add behavior assertions that disabled controls do not fire handlers.
## Review Comment

You already verify visual disabled state; adding `not.toHaveBeenCalled()` checks after click attempts would protect against regressions where disabled styling exists but events still fire.

## Triage

- Decision: `VALID`
- Notes: The read-only inventory test checked disabled attributes but did not prove disabled controls suppress callbacks. The fix clicks each disabled control and asserts the handlers were not called.
