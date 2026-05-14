---
provider: coderabbit
pr: "151"
round: 1
round_created_at: 2026-05-14T00:24:33.853673Z
status: resolved
file: internal/core/provider/coderabbit/coderabbit_test.go
line: 218
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6B7N8R,comment:PRRC_kwDORy7nkc7BA5WH
---

# Issue 004: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Wrap this scenario in a `t.Run("Should ...")` subtest.**

The new test currently executes assertions directly in the top-level function; please move it into a `Should...` subtest block to match the enforced test pattern.


As per coding guidelines, `**/*_test.go`: MUST use t.Run("Should...") pattern for ALL test cases.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/provider/coderabbit/coderabbit_test.go` around lines 203 - 218,
Wrap the test body of
TestWatchStatusFailsWhenCodeRabbitStatusFailsWithoutProviderReview in a t.Run
subtest whose name starts with "Should" (e.g., t.Run("Should return coderabbit
status failure", func(t *testing.T) { ... })), move the assertions and the call
to New(...).WatchStatus(...) into that subtest, and call t.Parallel()
appropriately (either keep the existing top-level t.Parallel() or call
t.Parallel() inside the subtest) so the test follows the required
t.Run("Should...") pattern while preserving the existing logic and referenced
symbols (TestWatchStatusFailsWhenCodeRabbitStatusFailsWithoutProviderReview,
New, WithCommandRunner, testWatchStatusRunnerWithStatuses, WatchStatus).
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - This file already mixes top-level test functions with `t.Run("Should ...")` subtests, and the neighboring malformed-payload coverage uses the `Should ...` subtest shape.
  - The new failure-path scenario is the outlier in the touched area because it performs its assertions directly in the top-level function.
  - Fix approach: wrap that single scenario in one `t.Run("Should ...")` subtest and keep the existing assertions unchanged.
  - Resolution: `TestWatchStatusFailsWhenCodeRabbitStatusFailsWithoutProviderReview` now executes through a `Should ...` subtest with the same assertions preserved.
