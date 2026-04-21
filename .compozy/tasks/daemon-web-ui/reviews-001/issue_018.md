---
status: resolved
file: internal/daemon/transport_mappers.go
line: 416
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqYJ,comment:PRRC_kwDORy7nkc651WIO
---

# Issue 018: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**`cloneTransportMetadataMap` still aliases nested metadata.**

This only clones the top-level `map[string]any`. Nested maps/slices remain shared with the source payload, so later mutation still leaks across layers despite the defensive copying elsewhere in this file.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/transport_mappers.go` around lines 409 - 416,
cloneTransportMetadataMap currently only shallow-copies the top-level map so
nested map[string]any and []any entries remain aliased; change it to perform a
recursive deep clone: for each value in cloneTransportMetadataMap (and helper
function e.g., deepCloneValue) detect types and recursively clone map[string]any
(calling cloneTransportMetadataMap) and []any (allocating a new slice and
deep-cloning each element), preserve primitive/immutable types by direct
assignment, and ensure nil/empty inputs return nil; update callers to use this
deep copy to prevent nested mutation leaks.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b81ebf2-33d3-49d0-b9c4-2c97e797915b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The transport mapper clones only the top-level metadata map, so nested values remain shared across layers even after the transport copy.
  - Root cause: `cloneTransportMetadataMap` is shallow while the daemon query metadata can contain nested maps/slices.
  - Intended fix: mirror the recursive deep-clone logic at the transport boundary so nested metadata stays isolated.

## Resolution

- Added recursive deep-cloning for nested transport metadata and covered the boundary behavior with regression tests.
- Verified with `make verify`.
