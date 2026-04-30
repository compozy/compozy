---
provider: coderabbit
pr: "133"
round: 3
round_created_at: 2026-04-30T21:50:44.830324Z
status: resolved
file: internal/cli/reviews_exec_daemon_additional_test.go
line: 584
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Ye_,comment:PRRC_kwDORy7nkc69AEHQ
---

# Issue 001: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Align new watch test cases with `t.Run("Should...")` naming policy**

The newly added watch tests are not consistently using the required `t.Run("Should...")` pattern (e.g., the case starting at Line 498 is not wrapped as a `Should...` subtest, and several new subtest names from Line 587 onward don’t use the `Should...` prefix). Please normalize these names for guideline compliance.

 

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".


Also applies to: 586-935

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/reviews_exec_daemon_additional_test.go` around lines 498 - 584,
Rename and wrap the standalone test function
TestReviewsWatchCommandBuildsDaemonRequest and all new watch subtests into the
t.Run("Should ...") pattern required by our guidelines: for
TestReviewsWatchCommandBuildsDaemonRequest, wrap its body in t.Run("Should build
daemon request for watch command", func(t *testing.T) { ... }) and similarly
rename and wrap each new subtest (those currently added after the
TestReviewsWatchCommandBuildsDaemonRequest and any subtests referenced between
the old ranges) to use descriptive "Should ..." titles; ensure you keep the
existing assertions and variables (e.g., client.startWatchReq, req, overrides,
batching) unchanged, only move them inside the new t.Run closures and update
subtest names to the "Should..." prefix consistently across all watch-related
tests.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1cee1f37-306f-41fe-860d-b9f818076e1c -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestReviewsWatchCommandBuildsDaemonRequest` is still a standalone test body, and several newer watch subtests use lowercase descriptive names instead of the repository-required `t.Run("Should...")` pattern.
- Fix approach: wrap the standalone body in a `Should...` subtest and rename the newer watch-related subtests to `Should...` titles without changing assertions or request wiring.
- Resolution: updated the watch command tests to use `Should...` subtests consistently and verified the batch with targeted tests plus `make verify`.
