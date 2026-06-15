---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/update_test.go
line: 238
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhqn,comment:PRRC_kwDORy7nkc7LlP3z
---

# Issue 018: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Wrap changed test scenarios with `t.Run("Should ...")` subtests to match test policy.**

These changed test cases are defined as direct top-level bodies instead of using the required `t.Run("Should...")` subtest pattern.

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases` and `Use table-driven tests with subtests (t.Run) as the default pattern`.  






Also applies to: 341-372, 461-491, 870-932

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/ui/update_test.go` around lines 136 - 238, The test
functions TestPauseSelectedJobEnablesComposerAndMessageResumes (lines 136-238),
and the other affected test functions at lines 341-372, 461-491, and 870-932 in
file internal/core/run/ui/update_test.go do not follow the required
t.Run("Should...") subtest pattern. For each of these test functions, wrap the
main test logic inside a t.Run call with a descriptive subtest name that
explains what the test should do (following the pattern t.Run("Should...",
func(t *testing.T) { ... })), rather than having the test body directly in the
top-level function. This ensures consistency with the coding guidelines that
require t.Run subtests for all test cases.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:2d9d564fa6d9d3a93f0cb75c -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: several updated UI tests in `update_test.go` contain direct top-level scenario bodies instead of `Should...` subtests.
- Fix approach: wrap each referenced scenario in a descriptive subtest and keep existing assertions unchanged.

## Resolution

- Resolved with scoped update test restructuring.
- Verification: `rtk make verify` exited 0 after the code changes.
