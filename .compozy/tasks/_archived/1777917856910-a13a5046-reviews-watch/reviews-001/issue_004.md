---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/core/provider/overlay_test.go
line: 192
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Dz,comment:PRRC_kwDORy7nkc68_V6F
---

# Issue 004: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Please convert this new test body to a `t.Run("Should ...")` subtest.**

That keeps it compliant with the enforced test structure used for all cases.

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/provider/overlay_test.go` around lines 169 - 192, Wrap the
existing test logic inside a t.Run subtest (e.g., t.Run("Should delegate watch
status to target", func(t *testing.T) { ... })) and move the setup/teardown
(ActivateOverlay and its defer restore) and the rest of the assertions into that
inner function so the outer TestAliasedProviderWatchStatusDelegatesToTarget only
contains the t.Run call; keep the same calls to ActivateOverlay, NewRegistry,
base.Register(&overlayTestProvider{name: "base"}), ResolveRegistry,
registry.Get("ext-review") and FetchWatchStatus to preserve behavior and
assertions.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ed821098-705a-4bc3-acaf-ab448a3674f2 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The new overlay watch-status test is a standalone body instead of using the required `t.Run("Should...")` subtest pattern.
- Fix plan: Wrap the test body in a single descriptive `Should ...` subtest while keeping the same overlay setup and assertions.
