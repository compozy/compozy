---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_watch_test.go
line: 810
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22ES,comment:PRRC_kwDORy7nkc68_V6q
---

# Issue 018: _🛠️ Refactor suggestion_ | _🟠 Major_ | _🏗️ Heavy lift_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _🏗️ Heavy lift_

**Adopt the required `t.Run("Should ...")` structure for all test cases.**

Most cases are standalone top-level tests; this file does not follow the repository’s required subtest pattern. Please wrap scenarios in `t.Run("Should ...")` blocks for consistency and maintainability.
 

As per coding guidelines, "**MUST use t.Run("Should...") pattern for ALL test cases**".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_watch_test.go` around lines 26 - 810, Wrap each
top-level test body in this file (e.g.
TestRunManagerReviewWatchCompletesCleanWithoutEmptyRound,
TestRunManagerReviewWatchPersistsRoundAndStartsOneChildRun,
TestRunManagerReviewWatchRejectsDuplicateActiveWatch, etc.) with a t.Run("Should
...", func(t *testing.T) { ... }) subtest using a human-readable "Should ..."
description derived from the test name, moving the current contents inside that
closure; preserve any existing inner t.Run calls (like the table-driven cases in
TestRunManagerReviewWatchFailureStates) and keep all setup/teardown, helper
calls, and assertions unchanged, only adding the t.Run wrapper to satisfy the
required subtest pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:fd59a763-64a2-4d66-ae65-58d4eb6327b4 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The new `review_watch_test.go` coverage was added as many standalone top-level tests, but repository policy requires scenario bodies to be wrapped in `t.Run("Should...")`.
- Fix plan: Wrap each top-level test body in a descriptive `Should ...` subtest while preserving existing helper calls and any nested subtests.
