---
status: resolved
file: internal/daemon/query_documents.go
line: 290
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqX5,comment:PRRC_kwDORy7nkc651WH9
---

# Issue 015: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Deep-clone front matter before caching/returning it.**

`cloneMetadataMap` only copies the top-level map. Nested YAML objects/slices remain shared, so mutating a returned document can still mutate the cached copy and leak state across requests. This needs a recursive clone for `map[string]any` / `[]any`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/query_documents.go` around lines 278 - 290, cloneMetadataMap
currently performs a shallow copy so nested maps/slices remain shared; update
cloneMetadataMap to perform a deep clone for map[string]any and []any by
recursively cloning any values that are maps (map[string]any) or slices ([]any)
and copying primitives by value; ensure nil is preserved and non-map/slice types
(strings, numbers, bools) are returned as-is. Use the same deep-clone helper
from cloneMetadataMap inside cloneMarkdownDocument so MarkdownDocument.Metadata
is fully deep-copied before caching/returning to avoid shared mutable state.
Make sure the function signature remains cloneMetadataMap(src map[string]any)
map[string]any and keep behavior of returning nil for empty/nil input.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e75280e4-7172-485f-934b-c3510e24ebf0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The document metadata clone only copies the top-level map, so nested objects/slices remain aliased and can mutate cached state across requests.
  - Root cause: `cloneMetadataMap` is shallow while the parsed YAML front matter can contain nested maps/slices.
  - Intended fix: implement a recursive deep clone for metadata maps/slices and use it on cache/store boundaries.

## Resolution

- Implemented recursive deep-cloning for nested metadata maps and slices and added regression coverage for nested frontmatter isolation.
- Verified with `make verify`.
