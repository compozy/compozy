---
provider: coderabbit
pr: "138"
round: 1
round_created_at: 2026-05-02T04:42:21.214086Z
status: pending
file: extensions/cy-qa-workflow/main_test.go
line: 203
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5_F_jj,comment:PRRC_kwDORy7nkc69T8ED
---

# Issue 003: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Adopt `t.Run("Should...")` consistently across all test scenarios.**

This file mixes direct top-level scenarios and non-`Should...` subtest names. Please convert scenarios to `t.Run("Should...")` (table-driven where applicable) so the suite matches the repository’s required test pattern.

 

As per coding guidelines, `**/*_test.go`: "MUST use t.Run("Should...") pattern for ALL test cases" and "Use table-driven tests with subtests (`t.Run`) as the default pattern."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@extensions/cy-qa-workflow/main_test.go` around lines 44 - 203, Convert every
top-level test scenario into t.Run subtests named with the "Should ..." prefix
and make table-driven cases use t.Run("Should ...") as well: wrap
TestEnsureQATasksCreatesReportAndExecutionTasks,
TestEnsureQATasksIsIdempotentForExistingQATasks,
TestMutateSessionRequestPrefixesGoalForQAExecution,
TestMutateSessionRequestDoesNotDuplicateExistingGoal, and
TestMutateSessionRequestSetsClaudeEffortForQAReport in t.Run("Should ...") calls
(keeping existing t.Parallel inside each subtest) and rename the subtest names
in TestRuntimeForTaskSelectsQARuntimes to "Should ..." variants while keeping
the same table-driven logic and assertions; ensure the unique identifiers
(ensureQATasks, runtimeForTask, mutateSessionRequest, reportMarker,
executionMarker, claudeEffortEnv) are used unchanged so only test scaffolding is
adjusted.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:0b6dfc2d-d4f0-411c-b015-f3d5f66fedc8 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
