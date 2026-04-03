---
status: resolved
file: internal/core/model/content_test.go
line: 305
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHB4,comment:PRRC_kwDORy7nkc60y0f2
---

# Issue 007: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Pin the failure mode in these negative tests.**

These checks only assert `err != nil`, so they still pass if `NewContentBlock` or `Marshal` starts failing for an unrelated reason. Please assert the expected message or error type for each case.


As per coding guidelines, "MUST have specific error assertions (ErrorContains, ErrorAs)."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/content_test.go` around lines 289 - 305, The negative
tests currently only check for non-nil errors, which is too broad; update the
assertions to pin the failure mode by asserting on specific error content or
types (use testing helpers like require.ErrorContains/require.ErrorIs or
t.Fatalf with strings). For calls to
model.NewContentBlock((*model.TextBlock)(nil)) and
model.NewContentBlock(struct{}{}), assert the returned error message or error
type matches the expected "nil pointer" and "unsupported payload" errors
respectively (use ErrorContains or ErrorIs). In TestContentBlockMarshalErrors,
replace the generic checks for json.Marshal(model.ContentBlock{}) and
json.Marshal(model.ContentBlock{Type: model.BlockText}) with assertions that the
error message contains the expected "missing type" and "missing data" texts (or
match the concrete error types if exported). Locate these checks in the test
function names TestContentBlockMarshalErrors and the NewContentBlock calls to
make the targeted changes.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:9505bd09-f35d-4087-8d05-bc08c4654a2d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: The negative constructor/marshal tests only assert `err != nil`, so they would still pass if those code paths started failing for the wrong reason. The tests should pin the expected failure mode.
- Fix plan: Replace the broad non-nil checks with specific `ErrorContains`-style assertions for the nil-pointer, unsupported-payload, missing-type, and missing-data cases.
- Resolution: Tightened the negative content-block tests to assert the expected error text for nil-pointer, unsupported-payload, missing-type, and missing-data failures. `make verify` passed.
