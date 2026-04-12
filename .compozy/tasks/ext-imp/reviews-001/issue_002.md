---
status: resolved
file: internal/cli/extensions_bootstrap.go
line: 278
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:b7f137875407
review_hash: b7f137875407
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 002: Consider preserving original values without trimming in cloneStringSlice.
## Review Comment

The function applies `strings.TrimSpace` to each value during cloning. If the intent is purely to clone, this mutates the data. If trimming is intentional for normalization, consider renaming to `cloneAndNormalizeStringSlice` for clarity.

## Triage

- Decision: `valid`
- Notes:
  - Root cause: the helper trims entries as part of overlay normalization, but its current name reads like a pure clone operation and hides that behavior from future callers.
  - Fix plan: keep the normalization behavior, but rename the helper and its call sites so the implementation and intent match.
  - Resolved: renamed the helper to reflect normalization semantics in `internal/cli/extensions_bootstrap.go`; verified with `make verify`.
