---
status: resolved
file: internal/core/extension/dispatcher_test.go
line: 390
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Rl8s,comment:PRRC_kwDORy7nkc621sgs
---

# Issue 007: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Assert the failure reason, not just `err != nil`.**

Both tests still pass if these calls start failing for an unrelated reason. Check a stable substring or typed error so the negative path stays pinned to the intended contract. 

As per coding guidelines: `**/*_test.go`: MUST have specific error assertions (ErrorContains, ErrorAs).


Also applies to: 430-433

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/dispatcher_test.go` around lines 388 - 390, The test
currently only checks err != nil for applyHookPatch and should assert the
specific failure reason; update the two occurrences that call applyHookPatch
(the one around applyHookPatch(map[string]any{"prompt_text": "base"},
json.RawMessage(`"bad"`) and the other at lines ~430-433) to use a specific
error assertion such as require.ErrorContains/require.ErrorAs (or
t.ErrorContains/t.ErrorAs) and match a stable substring or concrete error type
explaining why the patch is invalid (e.g., "invalid JSON type" or the specific
error returned by applyHookPatch) so the negative path is pinned to the intended
contract.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c1d29a9a-29d8-4bec-b3df-52129c8adbe5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The remaining invalid-patch negative test only checks `err != nil`, so any unrelated failure path would still satisfy the assertion.
  - I will pin the test to the intended contract by asserting the `applyHookPatch` wrapper context and the wrapped JSON type error, which keeps the failure reason specific even though the reviewed line numbers have shifted.
  - Resolved by asserting the stable `decode hook patch` wrapper text and the wrapped `*json.UnmarshalTypeError`.
