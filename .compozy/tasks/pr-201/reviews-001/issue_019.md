---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/ui/view_test.go
line: 60
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhqv,comment:PRRC_kwDORy7nkc7LlP4A
---

# Issue 019: _🛠️ Refactor suggestion_ | _🟠 Major_ | _🏗️ Heavy lift_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _🏗️ Heavy lift_

**Wrap the new test cases in `t.Run("Should...")` subtests.**

These new/rewritten tests assert directly in the top-level test body, and the token/sidebar table cases iterate without subtests. Please wrap the cases in `t.Run("Should...")` subtests, using table-driven subtests where applicable.

As per coding guidelines, `**/*_test.go`: “MUST use t.Run("Should...") pattern for ALL test cases” and “Use table-driven tests with subtests (`t.Run`) as the default pattern.”







Also applies to: 358-494, 584-742, 1077-1157, 1425-1480, 1544-1707

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/ui/view_test.go` around lines 38 - 60, The test function
TestJobsBodyFitsWindowWidthAcrossBreakpoints and the other test functions
referenced in the consolidated sites (at lines 358-494, 584-742, 1077-1157,
1425-1480, and 1544-1707) are not following the required "Should..." naming
pattern for subtests. Rename all t.Run calls across these test functions to use
descriptive "Should..." names instead of format-based names like "80x24". For
example, replace the format-string-based subtest name with a descriptive name
like "Should fit jobs body width at breakpoint" or similar that clearly explains
what the test case validates. This should be applied consistently across all the
mentioned test functions in the file to comply with the table-driven test
pattern with descriptive subtest naming requirement.
```

</details>

<!-- cr-comment:v1:600fb45d0b2ddef3e5fe5124 -->

_Source: Coding guidelines_

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: several view tests and table loops use top-level scenario bodies or terse subtest names rather than descriptive `Should...` subtests.
- Fix approach: wrap or rename the referenced scenarios to descriptive `Should...` subtests, including table-driven token/sidebar cases where each case proves a distinct rendering invariant.

## Resolution

- Resolved with scoped view test restructuring.
- Verification: `rtk make verify` exited 0 after the code changes.
