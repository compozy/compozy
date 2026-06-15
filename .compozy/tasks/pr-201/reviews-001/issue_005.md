---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/cli/tasks_run_wizard_test.go
line: 369
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhpt,comment:PRRC_kwDORy7nkc7LlP2p
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_ | _🏗️ Heavy lift_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _🏗️ Heavy lift_

**Refactor wizard tests to the required subtest pattern.**

The new test suite is implemented as many standalone top-level tests instead of `t.Run("Should...")`-based subtests/table-driven cases, which violates the repository test standards for `*_test.go`.





As per coding guidelines, `**/*_test.go` must use `t.Run("Should...")` for all test cases and use table-driven tests with subtests as the default pattern.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/cli/tasks_run_wizard_test.go` around lines 15 - 369, Refactor the
wizard tests to follow repository standards by consolidating the multiple
standalone top-level test functions
(TestTaskRunWizardModelSelectsMultipleWorkflowsAndSubmits,
TestTaskRunWizardModelPreservesAndReordersWorkflowSelection,
TestTaskRunWizardModelBuildsWorkflowScopedOverrides,
TestTaskRunWizardModelPreservesOverridesAcrossBackNavigation,
TestTaskRunWizardTextInputAcceptsGlobalShortcutCharacters,
TestTaskRunWizardRuntimeTextInputAcceptsNavigationLetters,
TestTaskRunWizardOverrideTextInputAcceptsNavigationLetters,
TestTaskRunWizardModelFiltersWorkflowSelection,
TestTaskRunWizardModelAcceptsManualWorkflowFallback, and
TestTaskRunWizardViewFitsTerminalBounds) into fewer parent test functions that
use t.Run("Should...") subtests for each test case. Implement table-driven test
patterns where applicable, such as for TestTaskRunWizardViewFitsTerminalBounds
which already has multiple dimensions to test. Each subtest should follow the
pattern t.Run("Should<describe_behavior>", func(t *testing.T) { ... }) and tests
should call t.Parallel() within each subtest as appropriate.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:7d97373be7c966411e374939 -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the new task-run wizard tests are mostly standalone top-level scenarios, and the existing table case uses terse dimension names rather than `Should...` subtest names.
- Fix approach: group the wizard behavior tests under parent test functions with descriptive `Should...` subtests, and rename the layout table subtests to descriptive `Should...` case names.

## Resolution

- Resolved with scoped wizard test restructuring.
- Verification: `rtk make verify` exited 0 after the code changes.
