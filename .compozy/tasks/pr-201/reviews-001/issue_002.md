---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/cli/form_test.go
line: 556
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhpd,comment:PRRC_kwDORy7nkc7LlP2U
---

# Issue 002: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Wrap these new scenarios in `t.Run("Should...")` subtests to match test conventions.**

Both new test cases run directly at top level. The repository test guidelines require `t.Run("Should...")` as the standard case structure.

As per coding guidelines, `*_test.go` must use the `t.Run("Should...")` pattern for all test cases (with table-driven subtests as default).  






Also applies to: 558-611

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/cli/form_test.go` around lines 495 - 556, The test functions
TestTaskRunFormInputsApplyMultipleWorkflowSelection and the other test case
mentioned in the consolidated sites are written as top-level test functions but
should follow the repository's testing convention by wrapping their test logic
in t.Run("Should...") subtests. Refactor each test function to use the t.Run
pattern where the subtest name describes the specific scenario being tested
(e.g., "Should apply multiple workflow selection correctly"), and move all the
test assertions and setup into the subtest body.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:a8d89e305a5a5ead5b35409c -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the two changed task-run form tests execute their scenario bodies directly at the top level instead of using `t.Run("Should...")` subtests.
- Fix approach: wrap each scenario body in a descriptive subtest while preserving the existing assertions and shared helpers.

## Resolution

- Resolved with scoped test restructuring.
- Verification: `rtk make verify` exited 0 after the code changes.
