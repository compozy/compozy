---
status: resolved
file: internal/daemon/run_transcript_test.go
line: 116
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUll,comment:PRRC_kwDORy7nkc68K-Ql
---

# Issue 034: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Use the repo-standard subtest pattern here.**

These cases cover real behavior, but the new test file skips the required `t.Run("Should...")` structure and doesn't mark the cases as parallel even though they're independent.


As per coding guidelines, `**/*_test.go`: "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "Use `t.Parallel()` for independent subtests."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/run_transcript_test.go` around lines 11 - 116, Wrap the two
existing test bodies (TestRunUIMessagesFromSessionPreservesStructuredEntries and
TestRunUIMessagesFromSessionMarksFailedTools) in explicit subtests using t.Run
with descriptive names (e.g., "preserves structured entries" and "marks failed
tools") and call t.Parallel() inside each subtest to mark them as independent;
keep the same setup and assertions and locate the bodies by the top-level test
function names and the call to runUIMessagesFromSession and helper
mustContractBlock to move their logic into the subtest closures.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:43567ed8-392f-4a75-9e7a-1958060562fd -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed the new transcript tests did not use subtests for their independent cases. Wrapped both test bodies in `t.Run("Should ...")` closures and marked the subtests parallel.
