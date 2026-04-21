---
status: resolved
file: internal/daemon/run_manager.go
line: 524
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:bc90aff144fc
review_hash: bc90aff144fc
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 020: Consider error handling for persistRuntimeIntegrity failure.
## Review Comment

While the warning log is appropriate, consider whether a persistence failure should affect the snapshot response. The current behavior continues despite the error, which may be intentional for resilience but could mask issues.

## Triage

- Decision: `invalid`
- Root cause analysis: `persistRuntimeIntegrity` only mirrors live runtime-integrity signals such as journal-drop counts back into the run database. It is best-effort metadata, not a prerequisite for building the snapshot payload itself.
- Why the finding does not apply: failing the snapshot response when this mirror step fails would hide otherwise-available run state from users and make a transient persistence problem look like total snapshot failure. The actual snapshot-integrity read/update path is still enforced by `loadRunIntegrity`, which already returns hard errors when required data cannot be loaded or updated.
- Resolution: no code change. The warning log plus continued snapshot response is the intended resilience behavior here.
