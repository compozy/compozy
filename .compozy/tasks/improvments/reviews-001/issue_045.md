---
status: resolved
file: web/src/routes/_app/stories/-runs.$runId.stories.tsx
line: 106
severity: minor
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:0352cff36801
review_hash: 0352cff36801
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 045: Error story should also fail transcript request for a fully degraded run-detail scenario.
## Review Comment

Right now Line 106 returns a successful transcript in the error story, which can mask transcript-error rendering behavior.

## Triage

- Decision: `VALID`
- Notes: The Storybook error scenario returned a failing snapshot but a successful transcript, so it did not exercise a fully degraded run-detail view. The fix makes the transcript handler return a matching 404 response.
