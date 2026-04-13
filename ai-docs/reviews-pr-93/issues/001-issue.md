# Issue 1 - Review Thread Comment

**File:** `internal/cli/extension/display_test.go:300`
**Date:** 2026-04-13 19:08:30 UTC
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Wrap these new cases in `t.Run("Should...")` subtests.**

The added tests are all standalone top-level cases right now, but the repo test policy requires the `Should...` subtest pattern for new test coverage. Please wrap each body in a named `t.Run(...)` and keep `t.Parallel()` inside the subtest where applicable.

As per coding guidelines `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

Also applies to: 302-312, 399-416, 901-916

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/display_test.go` around lines 280 - 300, The test
bodies (e.g., TestInstallPrintsSetupHintWhenExtensionShipsSetupAssets) must be
wrapped in named subtests using t.Run("Should ...") with the existing assertions
moved into the subtest body; move t.Parallel() inside each t.Run to keep
parallelism per-subtest, and update the other indicated test blocks (around
lines 302-312, 399-416, 901-916) the same way so every new test case follows the
repo-required t.Run("Should...") pattern.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f0410b06-af06-4312-8bf8-f831ab4cc296 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: wrapped the review-targeted `display_test.go` cases in named `t.Run("Should ...")` subtests and kept the parallel execution inside the subtests.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56n9hK`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56n9hK
```

---

_Generated from PR review - CodeRabbit AI_
