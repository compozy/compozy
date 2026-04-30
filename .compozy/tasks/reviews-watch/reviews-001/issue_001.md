---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/api/core/handlers_service_errors_test.go
line: 347
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Dg,comment:PRRC_kwDORy7nkc68_V5t
---

# Issue 001: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Use `Should...` phrasing for the new table-case names passed into `t.Run`.**

These names feed directly into `t.Run(tc.name, ...)`; align them with the required pattern for consistency.

<details>
<summary>Suggested diff</summary>

```diff
-			"review watch active conflict",
+			"Should return conflict when review watch is already active",
...
-			"review watch invalid request",
+			"Should return unprocessable when watch request is invalid",
...
-			"review watch unavailable",
+			"Should return service unavailable when watch is not available",
```
</details>

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers_service_errors_test.go` around lines 303 - 347,
The test case names used as tc.name for t.Run are not following the required
"Should..." pattern; update the three table entries whose name strings are
"review watch active conflict", "review watch invalid request", and "review
watch unavailable" to use descriptive "Should..." phrases (e.g., "Should return
conflict when review watch already active", "Should return unprocessable entity
for invalid watch request", "Should return service unavailable when review watch
is unavailable") so t.Run(tc.name, ...) uses the mandated naming convention.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ed821098-705a-4bc3-acaf-ab448a3674f2 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The three new watch-related table entries feed directly into `t.Run(tc.name, ...)` but do not follow the repository-mandated `Should...` naming convention.
- Fix plan: Rename only the cited case strings to descriptive `Should ...` phrases without changing handler behavior or assertions.
