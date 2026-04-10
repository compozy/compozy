---
status: resolved
file: internal/core/reviews/parser.go
line: 156
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWv,comment:PRRC_kwDORy7nkc61XmRY
---

# Issue 018: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Scope XML tag extraction to the `<review_context>` block.**

`extractXMLTag` scans the entire file, so body text that contains `<file>`, `<line>`, or similar markup can override the actual legacy metadata. Parse only within the `<review_context>...</review_context>` slice before reading tag values.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/reviews/parser.go` around lines 144 - 156, The extractXMLTag
function currently searches the whole file and must be limited to the
<review_context> block: first locate the "<review_context>" open and
"</review_context>" close (return "" if either is missing), slice content to
just that inner block, then perform the existing tag extraction logic on that
slice; update references to extractXMLTag to continue accepting the full file
string (the function itself will do the scoping), and keep the function name
extractXMLTag and tag parameter unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:a60f1eb0-d795-4bb2-8b4d-2afc11c2fe85 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `extractXMLTag` scans the entire review file, so XML-looking text in the body can override metadata that should only be sourced from `<review_context>`. That is a real parsing bug for legacy review files. The fix is to scope extraction to the review-context block first, then resolve tags only inside that slice.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
