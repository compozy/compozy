---
status: resolved
file: internal/daemon/transport_mappers.go
line: 436
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:d066d95dee60
review_hash: d066d95dee60
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 011: Consider logging metadata marshal failures for debugging.
## Review Comment

When `json.Marshal` fails (line 441-443), the error is silently discarded. While returning `nil` for optional metadata is reasonable, logging the error would help diagnose unexpected marshaling failures (e.g., unsupported types in metadata).

## Triage

- Decision: `invalid`
- Notes:
  - `marshalTransportMetadata` serializes metadata that is already normalized from YAML/frontmatter-derived `map[string]any` values and deep-cloned before transport mapping.
  - Adding `slog.Default()` warnings inside this pure mapper would introduce global logging side effects without request/run context and would not fix the real source if an unsupported value were ever introduced programmatically.
  - Analysis complete: no code change was made because if unsupported metadata types ever become possible, the root-cause fix belongs at metadata construction/validation boundaries rather than in transport serialization.
