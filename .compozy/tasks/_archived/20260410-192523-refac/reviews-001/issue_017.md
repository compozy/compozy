---
status: resolved
file: internal/core/prompt/prompt_test.go
line: 294
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWo,comment:PRRC_kwDORy7nkc61XmRP
---

# Issue 017: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Cover the hash/collision behavior in `TestSafeFileName`.**

This only checks the sanitized prefix, so a regression that drops or truncates the hash suffix still passes. Please make this table-driven with `t.Run(...)` and assert either the 6-character suffix shape or that two inputs with the same sanitized base produce different names.  


As per coding guidelines, "`**/*_test.go`: Use table-driven tests with subtests (`t.Run`) as the default pattern for Go tests" and "Ensure tests verify behavior outcomes, not just function calls."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/prompt/prompt_test.go` around lines 288 - 294, Update
TestSafeFileName to a table-driven subtest using t.Run and verify both the
sanitized prefix and the hash suffix behavior of SafeFileName: for each case
assert the returned string starts with the expected sanitized prefix and then
either (a) verify the suffix matches the expected 6-character alphanumeric
fingerprint shape (e.g., regex [A-Za-z0-9]{6} after a hyphen) or (b) include a
pair of inputs that sanitize to the same base and assert SafeFileName yields
different full names for them (ensuring a collision-resistant suffix); reference
the SafeFileName function and the TestSafeFileName test name when updating the
assertions.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d2033f55-fd4d-4209-b796-3f58621d2a7d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `TestSafeFileName` only checks the sanitized prefix, so it would miss regressions in the hash suffix or collision resistance. The test should become table-driven with `t.Run("Should ...")` cases that verify both the prefix and the deterministic fingerprint behavior.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
