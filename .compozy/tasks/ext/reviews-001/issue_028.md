---
status: resolved
file: internal/core/extension/host_reads_test.go
line: 163
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093239578,nitpick_hash:16b233a40d28
review_hash: 16b233a40d28
source_review_id: "4093239578"
source_review_submitted_at: "2026-04-11T02:29:46Z"
---

# Issue 028: Consider extracting the compaction threshold to a named constant.
## Review Comment

The magic number `180` for triggering compaction should ideally reference the same constant used in production code to prevent tests from drifting out of sync with the actual threshold.

## Triage

- Decision: `invalid`
- Notes: The test is not asserting the exact compaction boundary; it intentionally writes well past the workflow-memory threshold to verify the `NeedsCompaction` path. Replacing `180` with the production limit would add brittle coupling to an unexported implementation detail without improving behavioral coverage.
