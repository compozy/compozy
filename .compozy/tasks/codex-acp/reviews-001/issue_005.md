---
provider: coderabbit
pr: "151"
round: 1
round_created_at: 2026-05-14T00:24:33.853673Z
status: resolved
file: internal/daemon/review_watch_test.go
line: 190
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6B7N8S,comment:PRRC_kwDORy7nkc7BA5WJ
---

# Issue 005: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Add `t.Parallel()` in the new subtests if they are independent.**

Both added `t.Run("Should ...")` blocks look isolated and can likely run in parallel; adding `t.Parallel()` aligns with the test policy.


As per coding guidelines, `**/*_test.go`: Use `t.Parallel()` for independent subtests.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/daemon/review_watch_test.go` around lines 117 - 190, Both subtests
titled "Should fetch and fix unresolved reviews when provider settled without a
current review object" and "Should declare clean after provider settled current
head and no unresolved reviews remain" are independent and should be marked
parallel; add t.Parallel() as the first line inside each subtest's anonymous
func(t *testing.T) to enable concurrent execution and follow the test guideline.
Locate the two t.Run(...) blocks in internal/daemon/review_watch_test.go and
insert t.Parallel() at the top of each subtest function body (the funcs passed
to t.Run).
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The two added `current_settled` review-watch scenarios construct isolated fake providers, git state, and test environments; they do not share mutable state across subtests.
  - Adding `t.Parallel()` is safe here and matches the existing policy used throughout the daemon test suite for independent `Should ...` subtests.
  - Fix approach: mark those two subtests parallel without changing their assertions or fixtures.
  - Resolution: both `current_settled` review-watch subtests now call `t.Parallel()` at the start of the subtest body.
