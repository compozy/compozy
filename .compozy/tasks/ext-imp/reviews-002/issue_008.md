---
status: resolved
file: internal/core/workspace/config_validate.go
line: 47
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4094514795,nitpick_hash:c7fdde197eb0
review_hash: c7fdde197eb0
source_review_id: "4094514795"
source_review_submitted_at: "2026-04-12T04:17:10Z"
---

# Issue 008: Normalize output-format field names for consistent validation errors.
## Review Comment

`validateOutputFormatValue` here uses `start.output_format` / `fix_reviews.output_format`, while other validators in this file consistently prefix with `workspace config ...`. Aligning these keeps diagnostics uniform.

Based on learnings: Maintain consistency of patterns and conventions across the system.

Also applies to: 73-73

## Triage

- Decision: `invalid`
- Notes:
  - The current validation already reports the exact offending config keys (`start.output_format` and `fix_reviews.output_format`), so diagnostics remain precise.
  - Prefixing those messages with `workspace config` would make wording more uniform, but it does not correct a faulty validation path or ambiguous error.
  - No code change is planned because this is wording churn rather than a behavior defect.
