---
provider: coderabbit
pr: "133"
round: 3
round_created_at: 2026-04-30T21:50:44.830324Z
status: resolved
file: internal/core/sync_test.go
line: 686
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3YfR,comment:PRRC_kwDORy7nkc69AEHk
---

# Issue 006: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Use a specific error assertion for the invalid review-issue case.**

Line 684 only checks that an error exists. Assert a stable error fragment so the test validates failure reason, not just failure occurrence.

<details>
<summary>💡 Suggested fix</summary>

```diff
- if _, err := collectReviewRounds(workflowDir); err == nil {
- 	t.Fatal("expected invalid review issue to fail")
- }
+ if _, err := collectReviewRounds(workflowDir); err == nil ||
+ 	!strings.Contains(err.Error(), "review issue status is required") {
+ 	t.Fatalf("collectReviewRounds() error = %v, want missing status validation", err)
+ }
```
</details>

 

As per coding guidelines, "MUST have specific error assertions (ErrorContains, ErrorAs)".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if _, err := collectReviewRounds(workflowDir); err == nil ||
		!strings.Contains(err.Error(), "review issue status is required") {
		t.Fatalf("collectReviewRounds() error = %v, want missing status validation", err)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/sync_test.go` around lines 684 - 686, Replace the broad
nil-check on the error returned by collectReviewRounds with a specific assertion
that the error contains a stable fragment describing the failure (e.g., "invalid
review issue"); update the test so after calling
collectReviewRounds(workflowDir) you assert err != nil and that
strings.Contains(err.Error(), "invalid review issue") (or use your test helper
ErrorContains/require.ErrorContains helper) so the test validates the failure
reason rather than just presence of an error.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:fac9bd05-da51-494b-b380-cc1fbefdcd70 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the test only asserts that `collectReviewRounds` fails, so a different failure path could still satisfy the test and hide a regression in review-issue validation.
- Fix approach: tighten the assertion to check for the stable missing-status validation fragment emitted by the sync pipeline.
- Resolution: tightened the invalid-review assertion to match the current missing-status parse error and verified the batch with targeted tests plus `make verify`.
