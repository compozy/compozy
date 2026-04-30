---
provider: coderabbit
pr: "133"
round: 3
round_created_at: 2026-04-30T21:50:44.830324Z
status: resolved
file: internal/core/reviews/store_test.go
line: 406
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3YfD,comment:PRRC_kwDORy7nkc69AEHU
---

# Issue 005: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Mark this test helper with `t.Helper()`**

`reviewIssueContentWithRound` is a test helper but is not marked as such. Pass `*testing.T` in and call `t.Helper()` so failure traces point at the caller test.

<details>
<summary>Suggested change</summary>

```diff
-func reviewIssueContentWithRound(status string, prLine string, createdAt time.Time) string {
+func reviewIssueContentWithRound(t *testing.T, status string, prLine string, createdAt time.Time) string {
+	t.Helper()
 	lines := []string{
 		"---",
 		"provider: coderabbit",
 	}
```

```diff
-[]byte(reviewIssueContentWithRound("resolved", tc.prLine, tc.createdAt)),
+[]byte(reviewIssueContentWithRound(t, "resolved", tc.prLine, tc.createdAt)),
```

```diff
-[]byte(reviewIssueContentWithRound("pending", tc.prLine, tc.createdAt)),
+[]byte(reviewIssueContentWithRound(t, "pending", tc.prLine, tc.createdAt)),
```
</details>

 

As per coding guidelines, "Mark test helper functions with t.Helper() so stack traces point to the caller."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/reviews/store_test.go` around lines 385 - 406, Convert the test
helper reviewIssueContentWithRound to accept a *testing.T parameter and call
t.Helper() at the top of the function so failures point to the caller; update
any test callers that invoke reviewIssueContentWithRound to pass their
*testing.T. Specifically, change the signature of reviewIssueContentWithRound to
include t *testing.T, add t.Helper() as the first statement inside the function,
and update all usages in the tests to pass the test instance.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:e2361daf-da86-4711-8790-353d73e63399 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Reasoning: `reviewIssueContentWithRound` is a pure string-construction helper. It does not receive `*testing.T`, does not report failures, and cannot improve stack traces via `t.Helper()`. Threading `*testing.T` through every call would add noise without changing failure attribution or test behavior.
- No code change: leave the helper as a pure value builder and close this issue after batch verification.
- Resolution: no code change required; full `make verify` passed with the helper left as a pure string builder.
