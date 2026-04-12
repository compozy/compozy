---
status: resolved
file: internal/core/agent/registry_validate.go
line: 128
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:02df24ff4919
review_hash: 02df24ff4919
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 006: Consider extracting format normalization into a helper.
## Review Comment

The format normalization logic (empty → `OutputFormatText`) is duplicated in both `validateRuntimeOutputFormat` and `validateRuntimeExecMode`. While acceptable since both functions may be called independently, a small helper like `normalizedFormat(format model.OutputFormat) model.OutputFormat` would reduce drift risk if the default changes.

## Triage

- Decision: `invalid`
- Notes:
  - The duplicated normalization is currently a two-line defaulting rule inside two independently-invoked validators. Keeping it local keeps each validation path self-contained and avoids introducing a helper with no behavior beyond a single `if format == ""`.
  - There is no observable bug here today, so I am not changing production code for a style-only extraction.
