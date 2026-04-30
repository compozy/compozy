---
status: resolved
file: internal/core/run/executor/execution_acp_integration_test.go
line: 32
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:02672972cb6e
review_hash: 02672972cb6e
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 024: Nice timeout consolidation, but the constant name is now slightly too narrow.
## Review Comment

`runACPHelperHappyPathTimeout` is reused in retry/non-happy-path tests too; consider renaming to something neutral like `runACPHelperDefaultTimeout` to keep intent clear.

Also applies to: 70-70, 168-168, 228-228, 289-289, 597-597, 635-635

## Triage

- Decision: `valid`
- Notes: Confirmed `runACPHelperHappyPathTimeout` was reused by retry and error-path ACP integration tests. Renamed it to `runACPHelperDefaultTimeout` to match its broader purpose.
