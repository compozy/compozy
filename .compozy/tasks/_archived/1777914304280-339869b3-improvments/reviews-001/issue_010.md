---
status: resolved
file: internal/api/httpapi/browser_middleware_test.go
line: 66
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUk-,comment:PRRC_kwDORy7nkc68K-Pw
---

# Issue 010: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Use `t.Run("Should ...")` names for these subtests.**

Please rename subtests to the required `Should ...` pattern to align with test conventions.

  
As per coding guidelines, `**/*_test.go`: MUST use `t.Run("Should...")` pattern for ALL test cases.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/browser_middleware_test.go` around lines 57 - 66, Update
the subtest names in the testCases slice to follow the required t.Run("Should
...") pattern: change the name values in the testCases declaration (the struct
fields name, rootDir, wantErr) to descriptive strings starting with "Should ..."
(for example: "Should accept valid directory", "Should fail on empty root",
"Should fail on missing root", "Should fail when root is a file"); leave rootDir
and wantErr values unchanged so existing t.Run(test.name, ...) calls keep
working.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:0aa58313-6e22-4f13-a85f-3db5cd1d7a6e -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: The `validateWorkspaceRoot` table names did not follow the required `Should...` convention. Renamed all four cases while keeping the same inputs.
