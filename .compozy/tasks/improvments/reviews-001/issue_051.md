---
status: resolved
file: web/src/systems/workflows/components/workflow-inventory-view.tsx
line: 94
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:414b7a9ff917
review_hash: 414b7a9ff917
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 051: Avoid hardcoding the reason for read-only mode.
## Review Comment

`isReadOnly` only tells this component that writes are disabled, but the alert hardcodes "Workspace path missing". If the workspace becomes read-only for another validation reason, the banner will show the wrong diagnosis. Use a neutral message here, or pass the specific reason down.

## Triage

- Decision: `VALID`
- Notes: The component only receives `isReadOnly`, not the specific reason writes are disabled, so hardcoding "Workspace path missing" can display a wrong diagnosis. The fix uses a neutral read-only message.
