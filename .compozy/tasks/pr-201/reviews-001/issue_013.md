---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/model_test.go
line: 103
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhqh,comment:PRRC_kwDORy7nkc7LlP3u
---

# Issue 013: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Wrap each job-state assertion in `t.Run("Should...")` subtests.**

Convert the state loop into named subtests so each state failure is isolated and compliant with the repository test policy.

As per coding guidelines: `MUST use t.Run("Should...") pattern for ALL test cases` and `Use table-driven tests with subtests (t.Run) as the default pattern`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/ui/model_test.go` around lines 93 - 103, The test loop
iterating over jobState values (jobPending, jobRunning, jobRetrying, jobSuccess,
jobFailed) needs to be refactored to use the t.Run pattern with named subtests.
For each state in the loop, wrap the existing assertions for jobStateIcon,
jobStateColor, and jobBorderColor inside a t.Run call with a descriptive name
following the "Should..." convention (for example, naming the subtest based on
the current state being tested). This isolates each state's test results and
complies with the repository's test policy requiring t.Run subtests for all test
cases.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:324222d13c1b975d0c3b3bd2 -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestFormattingAndStateHelpersCoverBranches` loops over job states without subtests, so a per-state helper failure is not isolated.
- Fix approach: wrap each job-state helper assertion group in a `Should...` subtest named for the state under test.

## Resolution

- Resolved with per-state `Should...` subtests.
- Verification: `rtk make verify` exited 0 after the code changes.
