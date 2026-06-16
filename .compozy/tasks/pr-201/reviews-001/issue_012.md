---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/transcript/model_test.go
line: 109
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6JqhqV,comment:PRRC_kwDORy7nkc7LlP3c
---

# Issue 012: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Use the required subtest wrapper pattern for this case.**

Line 74 introduces a top-level test body instead of the required `t.Run("Should...")` structure for test cases in `_test.go` files.






As per coding guidelines, `**/*_test.go`: “MUST use t.Run("Should...") pattern for ALL test cases”.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/transcript/model_test.go` around lines 74 - 109, The test
function TestSessionViewModelAppendsUserMessagesAsDistinctEntries is structured
as a top-level test body instead of following the required subtest wrapper
pattern. Wrap the entire test logic inside a t.Run() call using a descriptive
"Should..." naming convention (for example, "Should append user messages as
distinct entries"), and move the test body into the callback function passed to
t.Run(). This ensures consistency with the project's coding guidelines for all
test cases in _test.go files.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:3105a685771ff1f07322d5f7 -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestSessionViewModelAppendsUserMessagesAsDistinctEntries` has its scenario body directly in the top-level test function.
- Fix approach: wrap the scenario in a descriptive `t.Run("Should...")` subtest while preserving the assertions.

## Resolution

- Resolved with scoped test restructuring.
- Verification: `rtk make verify` exited 0 after the code changes.
