---
status: resolved
file: internal/api/core/interfaces.go
line: 331
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqWd,comment:PRRC_kwDORy7nkc651WF_
---

# Issue 004: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

# Check whether MarkdownDocument.Metadata is populated with a stable shape.
rg -n -C3 '\bMarkdownDocument\b' --type go
rg -n -C3 '\bMetadata\s*:' --type go
rg -n -C3 '\.Metadata\b' --type go
```

Repository: compozy/compozy

Length of output: 50371

---



</details>

**Avoid `map[string]any` in this transport contract.**

`MarkdownDocument` is an exported API payload and `Metadata map[string]any` leaves the schema unspecified for OpenAPI and SPA code generation. The daemon sources metadata from YAML frontmatter, which is a stable shape—model it explicitly in the API contract or use `json.RawMessage` if flexibility is required.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/interfaces.go` around lines 325 - 331, The exported type
MarkdownDocument uses Metadata map[string]any which prevents schema generation;
change the Metadata field in MarkdownDocument to a well-defined type (either a
concrete FrontMatter struct that models the YAML frontmatter fields used by the
daemon, e.g. author/date/tags/etc., or json.RawMessage if you need opaque
flexibility) and update any code that reads/writes Metadata to marshal/unmarshal
to that new type; ensure you add the json import if using json.RawMessage and
update references to MarkdownDocument.Metadata across the codebase so the
transport contract is explicit and OpenAPI/SPA generation will work.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:a9f5b26c-6acb-406f-a7a4-c71fefe05a3a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `core.MarkdownDocument.Metadata` is part of the browser-facing transport contract and currently generates an unusable nested-object OpenAPI shape from `map[string]any`.
  - Root cause: the daemon-side flexible metadata map is exposed directly at the transport layer instead of being encoded as an explicit opaque JSON payload.
  - Intended fix: keep daemon-side metadata parsing as a map, but change the public transport contract to an explicit JSON representation and update the transport/spec artifacts accordingly.

## Resolution

- Changed the browser-facing `MarkdownDocument.Metadata` transport field to `json.RawMessage`, marshaled transport metadata explicitly, and updated the OpenAPI document plus generated TypeScript types.
- Verified with `make verify`.
