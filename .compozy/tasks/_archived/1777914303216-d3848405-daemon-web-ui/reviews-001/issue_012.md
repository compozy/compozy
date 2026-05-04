---
status: resolved
file: internal/api/httpapi/static.go
line: 85
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqXY,comment:PRRC_kwDORy7nkc651WHL
---

# Issue 012: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Return 404 for static directories instead of falling back to `index.html`.**

When `fs.Stat` finds a directory like `/assets`, `resolveAsset()` returns `false` and `shouldServeSPAIndex()` still returns `true` because the last path segment has no dot. That makes `/assets` respond with the SPA shell instead of a static-path 404.



Also applies to: 121-125

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/static.go` around lines 83 - 85, resolveAsset currently
treats a directory hit (info.IsDir()) as a "not found" by returning "", false,
which causes shouldServeSPAIndex to think the URL is a missing file and serve
the SPA index; instead, change resolveAsset so that when fs.Stat finds a
directory it returns the empty asset path but signals "static path exists" (for
example return "", true) so the caller will return a 404 for static directories
rather than falling back to the SPA index; apply the same change to the
identical check at the other location (the block around lines 121-125) and keep
the rest of resolveAsset/shouldServeSPAIndex logic unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b81ebf2-33d3-49d0-b9c4-2c97e797915b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Static directory requests such as `/assets` are currently treated as SPA deep links because directory hits are collapsed into “missing asset.”
  - Root cause: `resolveAsset` returns the same `(empty, false)` result for both missing paths and existing directories.
  - Intended fix: distinguish existing directories from missing files so directory paths return 404 instead of `index.html`.

## Resolution

- Taught `resolveAsset` to distinguish existing directories from missing files so directory requests return `404`, and added regression coverage for `/assets`.
- Verified with `make verify`.
