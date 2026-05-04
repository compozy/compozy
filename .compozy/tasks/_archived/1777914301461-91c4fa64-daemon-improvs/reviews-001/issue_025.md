---
status: resolved
file: internal/store/globaldb/close_test.go
line: 42
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go8s,comment:PRRC_kwDORy7nkc651UNV
---

# Issue 025: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Use the required `t.Run("Should...")` test-case pattern.**

Please wrap this case in a named subtest (and table-drive if additional scenarios are added) to match repository test conventions.


As per coding guidelines, `**/*_test.go`: "MUST use t.Run("Should...") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/close_test.go` around lines 11 - 42, Wrap the
existing TestGlobalDBCloseContextDelegatesToSQLiteCloser logic inside a named
subtest using t.Run with a "Should..." description (e.g., t.Run("Should
propagate caller context and close underlying sqlite DB", func(t *testing.T) {
... })), keeping the existing setup that overrides closeGlobalSQLiteDatabase and
assertions against gotCtx, gotDB and global.closed; if you anticipate more
scenarios, convert to a table-driven loop and call t.Run for each case, but at
minimum enclose the current test body in a single t.Run("Should...") subtest to
match repository test conventions.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:bc146171-8676-4e63-8078-ba462713e655 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestGlobalDBCloseContextDelegatesToSQLiteCloser` is a single unnamed case even though repository conventions require `t.Run("Should...")` subtests for test cases.
- Fix approach: wrap the current assertions in named subtests and extend the file with close-retry coverage needed for the associated close-path fix.
- Resolution: `internal/store/globaldb/close_test.go` now uses named `Should...` subtests and includes retry-after-failure coverage alongside the delegation case.
- Verification: `go test ./internal/store/globaldb` and `make verify`
