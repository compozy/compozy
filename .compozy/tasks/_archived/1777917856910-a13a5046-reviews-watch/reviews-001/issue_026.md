---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: pkg/compozy/events/event_test.go
line: 397
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Et,comment:PRRC_kwDORy7nkc68_V7O
---

# Issue 026: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Use the required `Should...` subtest naming pattern.**

The new case name should follow the enforced `t.Run("Should...")` format.

<details>
<summary>Suggested change</summary>

```diff
-			name: "review watch lifecycle",
+			name: "Should round-trip review watch lifecycle payload",
```
</details>

 

As per coding guidelines: `MUST use t.Run("Should...") pattern for ALL test cases`.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
			name: "Should round-trip review watch lifecycle payload",
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/events/event_test.go` at line 397, Rename the table-driven test
case's name field to follow the required t.Run("Should...") pattern: change the
case currently named "review watch lifecycle" to a name starting with "Should",
e.g. "Should review watch lifecycle", so the test invocation t.Run(...) conforms
to the enforced naming convention; update the entry in the test cases slice in
event_test.go where the name key is defined (the test case that currently reads
"review watch lifecycle").
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:4be89c01-af65-405b-8c51-05f7eb643ca0 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: The table entry name is wired directly into `t.Run`, so the current `"review watch lifecycle"` label misses the enforced `Should...` convention. I will rename the case without changing the payload coverage.
- Resolution: Renamed the table case to the required `Should...` form and reverified it in the full test pipeline.
