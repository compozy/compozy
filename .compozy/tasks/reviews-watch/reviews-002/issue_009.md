---
provider: coderabbit
pr: "133"
round: 2
round_created_at: 2026-04-30T21:47:34.803875Z
status: pending
file: internal/daemon/review_watch_git_test.go
line: 239
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Yfh,comment:PRRC_kwDORy7nkc69AEH4
---

# Issue 009: _🛠️ Refactor suggestion_ | _🟠 Major_ | _🏗️ Heavy lift_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _🏗️ Heavy lift_

**Adopt `t.Run("Should...")` consistently across test cases**

Most cases are single top-level tests rather than `t.Run("Should...")` subtests. Please align the suite to the repository’s required test pattern for consistency and maintainability.

 

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_watch_git_test.go` around lines 11 - 239, The tests in
this file (e.g., TestReviewWatchGitStateReadsOnlyRepositoryState,
TestReviewWatchGitPushUsesOnlyAllowedCommandShape,
TestReviewWatchGitPushRejectsMissingTarget,
TestReviewWatchGitPushWrapsCommandFailure,
TestReviewWatchGitCommandRunnerAndParsers,
TestReviewWatchGitStateWithoutUpstreamStillReportsHead,
TestReviewWatchGitStateValidatesRunnerWorkspaceAndRequiredReads,
TestReviewWatchGitPushValidatesRunnerAndWorkspace) must follow the repository
pattern of using t.Run("Should...") for each case; wrap each test body inside a
top-level t.Run call with a descriptive "Should ..." name (keeping t.Parallel()
inside the subtest where needed) or convert the test to call t.Run(...)
immediately after t.Parallel(), ensuring existing assertions and helper setup
(like the execReviewWatchGit run closures and subtests in
TestReviewWatchGitStateValidatesRunnerWorkspaceAndRequiredReads) remain
unchanged except for being enclosed in the t.Run subtest closure so the suite
consistently uses the t.Run("Should...") pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:a02def14-ace0-4527-9531-2aec99eb5414 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
