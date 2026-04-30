---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: pkg/compozy/events/kinds/payload_compat_test.go
line: 139
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Ew,comment:PRRC_kwDORy7nkc68_V7R
---

# Issue 027: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Wrap the new compatibility case in a `t.Run("Should ...")` subtest.**

The assertions are good; this is just to satisfy the required test-case structure.

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/events/kinds/payload_compat_test.go` around lines 91 - 139, Wrap
the existing assertions in TestReviewWatchPayloadJSONCompatibility inside a
t.Run subtest with a descriptive "Should ..." name (e.g., t.Run("Should
serialize ReviewWatchPayload to stable JSON map", func(t *testing.T) { ... })),
move the t.Parallel() call into the subtest body (call t.Parallel() as the first
line inside the func), and keep all existing setup, the payload variable,
mustMarshalMap(t, payload) call, and the reflect.DeepEqual check unchanged
inside that subtest so the test logic and assertions remain identical.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ed821098-705a-4bc3-acaf-ab448a3674f2 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: `TestReviewWatchPayloadJSONCompatibility` currently runs as a single flat test body instead of the required `t.Run("Should ...")` subtest form. I will wrap the existing assertions in a subtest and keep the JSON-compatibility behavior unchanged.
- Resolution: Wrapped the compatibility assertions in a `Should ...` subtest, preserved the payload expectations, and reverified with `make verify`.
