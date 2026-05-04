---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_watch_test.go
line: 743
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22EU,comment:PRRC_kwDORy7nkc68_V6s
---

# Issue 017: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Strengthen negative-path assertions with specific error checks.**

Several checks only assert `err != nil`; please assert error content/type so failures verify the intended validation branch (not just any error).
 

<details>
<summary>Suggested assertion pattern</summary>

```diff
- if _, err := resolveReviewWatchOptions(workspacecfg.ProjectConfig{}, apicore.ReviewWatchRequest{PRRef: "123"}); err == nil {
- 	t.Fatal("missing provider error = nil, want validation error")
- }
+ if _, err := resolveReviewWatchOptions(workspacecfg.ProjectConfig{}, apicore.ReviewWatchRequest{PRRef: "123"}); err == nil || !strings.Contains(err.Error(), "provider") {
+ 	t.Fatalf("missing provider error = %v, want provider validation error", err)
+ }
```
</details>

As per coding guidelines, "**MUST have specific error assertions (ErrorContains, ErrorAs)**".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_watch_test.go` around lines 726 - 743, The tests
currently only assert that resolveReviewWatchOptions and
resolveReviewWatchDuration return a non-nil error; change them to assert
specific error content/types so they validate the intended validation branch:
for resolveReviewWatchOptions calls (with apicore.ReviewWatchRequest{PRRef:
"123"} and {Provider: "coderabbit"}) assert the returned error is the expected
validation error (use errors.Is or test helper ErrorAs/ErrorContains with the
known validation error sentinel/message), and for resolveReviewWatchDuration
calls assert the error contains the expected message for zero duration and for
invalid parse (use ErrorContains or strings.Contains on the error string or
errors.Is if there’s a sentinel). Reference the functions
resolveReviewWatchOptions and resolveReviewWatchDuration and the input shapes
apicore.ReviewWatchRequest to locate tests to update.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:fd59a763-64a2-4d66-ae65-58d4eb6327b4 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The validation tests currently accept any non-nil error, so they can pass even if the wrong validation branch fires.
- Fix plan: Tighten the assertions to check the expected provider, `pr_ref`, and duration error messages explicitly.
