---
provider: coderabbit
pr: "138"
round: 2
round_created_at: 2026-05-02T04:56:54.019903Z
status: pending
file: sdk/extension/extension_test.go
line: 298
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5_GD6a,comment:PRRC_kwDORy7nkc69UBlW
---

# Issue 005: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Wrap this scenario in a `t.Run("Should...")` subtest**

This new test should use the mandatory `t.Run("Should...")` pattern for test cases.




<details>
<summary>Minimal adjustment</summary>

```diff
 func TestOnAgentPreSessionCreateReceivesReadablePromptAndReturnsReadablePatch(t *testing.T) {
 	t.Parallel()
+	t.Run("Should receive readable prompt and return readable session patch", func(t *testing.T) {
+		t.Parallel()
 
-	const name = "sdk-ext"
-	const version = "1.0.0"
-	seen := make(chan extension.AgentPreSessionCreatePayload, 1)
-	ext := extension.New(name, version).
+		const name = "sdk-ext"
+		const version = "1.0.0"
+		seen := make(chan extension.AgentPreSessionCreatePayload, 1)
+		ext := extension.New(name, version).
 		WithCapabilities(extension.CapabilityAgentMutate).
 		OnAgentPreSessionCreate(func(
 			_ context.Context,
@@
-	shutdownHarness(ctx, t, harness, errCh)
+		shutdownHarness(ctx, t, harness, errCh)
+	})
 }
```
</details>

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/extension/extension_test.go` around lines 213 - 298, The test
TestOnAgentPreSessionCreateReceivesReadablePromptAndReturnsReadablePatch must be
wrapped in a subtest using t.Run("Should ..."); modify the test by replacing the
top-level body with a single t.Run call (e.g., t.Run("Should receive readable
prompt and return readable patch", func(t *testing.T) { ... })) and move the
existing contents inside that func; ensure t.Parallel() is called inside the
subtest (not before t.Run), and keep calls and references intact (e.g.,
runHarnessedExtension, harness.Initialize, harness.DispatchHook,
shutdownHarness) so setup/teardown and assertions remain unchanged.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:0ad89fc6-58e3-486d-a0d8-db7a327c49c6 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
