---
status: pending
file: internal/cli/commands_test.go
line: 103
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Tdvx,comment:PRRC_kwDORy7nkc624Dt0
---

# Issue 001: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Align subtest names with the required `Should...` convention.**

Please rename `t.Run(tc.name, ...)` to a `Should...` phrasing per test standards.


<details>
<summary>Minimal naming adjustment</summary>

```diff
-		t.Run(tc.name, func(t *testing.T) {
+		t.Run("Should default --tui to true for "+tc.name, func(t *testing.T) {
```
</details>

As per coding guidelines: `**/*_test.go`: “MUST use t.Run("Should...") pattern for ALL test cases”.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/commands_test.go` around lines 80 - 103, The subtests in
TestStartAndFixReviewsCommandsDefaultTUIToTrue use t.Run(tc.name, ...) which
violates the test naming convention; update the t.Run calls to use "Should..."
phrasing (e.g., t.Run("Should set default --tui to true for start", ...) and
t.Run("Should set default --tui to true for fix-reviews", ...)) while leaving
the test body and assertions unchanged; locate the test function
TestStartAndFixReviewsCommandsDefaultTUIToTrue and the table entries created via
newStartCommandWithDefaults and newFixReviewsCommandWithDefaults and replace the
t.Run invocations that reference tc.name with appropriate "Should..."
descriptions.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:5012106a-87cb-4ead-9a79-79d0f79ccbda -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
