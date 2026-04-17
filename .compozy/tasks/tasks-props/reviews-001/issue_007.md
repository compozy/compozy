---
status: resolved
file: internal/core/model/task_runtime.go
line: 160
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4130406042,nitpick_hash:bcc0d902f972
review_hash: bcc0d902f972
source_review_id: "4130406042"
source_review_submitted_at: "2026-04-17T16:20:53Z"
---

# Issue 007: Unexpected trimming in cloneOptionalString.
## Review Comment

The `cloneOptionalString` function trims whitespace during cloning (line 164), which is a side effect that might be unexpected for a function named "clone". While this is likely intentional for normalization, consider either:
1. Renaming to `cloneAndTrimOptionalString` for clarity, or
2. Adding a brief comment explaining the trimming behavior

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `cloneOptionalString` trims whitespace as part of cloning, so the helper does more than its current name suggests.
  - Root cause: the normalization step is intentional for task-runtime rule copies, but the current name hides that behavior and makes later callers easier to misread.
  - Intended fix: rename the helper to reflect normalization while preserving the existing trimmed-clone behavior.
  - Resolution: the helper is now named `cloneTrimmedOptionalString`, which makes the normalization behavior explicit at each call site.
