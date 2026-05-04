---
status: resolved
file: pkg/compozy/runs/integration_test.go
line: 419
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4192176383,nitpick_hash:6e3e407d3494
review_hash: 6e3e407d3494
source_review_id: "4192176383"
source_review_submitted_at: "2026-04-28T20:30:08Z"
---

# Issue 044: Return a copied NextCursor to avoid shared mutable state leakage.
## Review Comment

At Line 429, the method returns the snapshot pointer directly. Copying it avoids accidental mutation coupling in tests.

## Triage

- Decision: `VALID`
- Notes: `integrationRunService.Transcript` returned `snapshot.NextCursor` directly. Because the cursor is a pointer, callers could mutate shared service fixture state. The fix copies the pointed-to cursor before returning it.
