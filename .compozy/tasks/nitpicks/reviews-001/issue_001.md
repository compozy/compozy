---
status: resolved
file: internal/cli/form_test.go
line: 154
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56KkLx,comment:PRRC_kwDORy7nkc62sRi8
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Wrap this new test case in `t.Run("Should...")` to match test policy.**

The assertion is good, but this new case should follow the required subtest pattern.

<details>
<summary>Suggested adjustment</summary>

```diff
 func TestFetchReviewsFormIncludesNitpicksToggle(t *testing.T) {
 	t.Parallel()
 
-	keys := formFieldKeys(newFetchReviewsCommand(nil), newCommandState(commandKindFetchReviews, core.ModePRReview))
-
-	assertFieldKeysPresent(t, keys, "name", "provider", "pr", "round", "nitpicks")
+	t.Run("Should include nitpicks toggle in fetch-reviews form", func(t *testing.T) {
+		t.Parallel()
+		keys := formFieldKeys(newFetchReviewsCommand(nil), newCommandState(commandKindFetchReviews, core.ModePRReview))
+		assertFieldKeysPresent(t, keys, "name", "provider", "pr", "round", "nitpicks")
+	})
 }
```
</details>

As per coding guidelines, `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestFetchReviewsFormIncludesNitpicksToggle(t *testing.T) {
	t.Parallel()

	t.Run("Should include nitpicks toggle in fetch-reviews form", func(t *testing.T) {
		t.Parallel()
		keys := formFieldKeys(newFetchReviewsCommand(nil), newCommandState(commandKindFetchReviews, core.ModePRReview))
		assertFieldKeysPresent(t, keys, "name", "provider", "pr", "round", "nitpicks")
	})
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form_test.go` around lines 148 - 154, Wrap the
TestFetchReviewsFormIncludesNitpicksToggle body in a t.Run subtest with a
"Should..." name to follow the t.Run pattern: call t.Run("Should include
nitpicks toggle in fetch reviews form", func(t *testing.T) { ... }) and move the
existing parallelization, setup (newFetchReviewsCommand(nil),
newCommandState(commandKindFetchReviews, core.ModePRReview)) and the assertion
assertFieldKeysPresent(t, keys, "name", "provider", "pr", "round", "nitpicks")
inside that func; keep t.Parallel() at the start of the top-level test and/or
inside the subtest as appropriate.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:dbc050c4-4733-4672-b5ca-0278ed2a94f2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestFetchReviewsFormIncludesNitpicksToggle` is a standalone top-level assertion block and does not follow the repository's required `t.Run("Should ...")` subtest convention used for new Go test cases.
- Impact: This keeps the newly added fetch-reviews coverage out of alignment with the repo's enforced test structure, which makes this file inconsistent with the expected table/subtest style.
- Fix approach: Wrap the existing assertions in a single `t.Run("Should ...")` subtest and keep the top-level `t.Parallel()` so the test matches the local policy without changing behavior.
- Resolution: `TestFetchReviewsFormIncludesNitpicksToggle` now wraps the nitpicks-toggle assertion in a `t.Run("Should ...")` subtest while preserving the original behavior and parallel top-level test execution.
- Verification: `go test ./internal/cli ./internal/core ./internal/core/provider/coderabbit` passed, and `env -u COMPOZY_NO_UPDATE_NOTIFIER make verify` passed cleanly.
