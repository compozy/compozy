---
status: resolved
file: internal/core/run/internal/acpshared/command_io_test.go
line: 194
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzN,comment:PRRC_kwDORy7nkc644Ms1
---

# Issue 010: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Don’t run the factory-swap test in parallel.**

`SwapNewAgentClientForTest` changes package-global constructor state. With `t.Parallel()` here, any other parallel test that creates ACP clients can observe the wrong factory and flake.

<details>
<summary>Suggested fix</summary>

```diff
 func TestCreateACPClientUsesPerJobRuntimeWhenPresent(t *testing.T) {
-	t.Parallel()
-
 	var captured agent.ClientConfig
 	restore := SwapNewAgentClientForTest(func(_ context.Context, cfg agent.ClientConfig) (agent.Client, error) {
```
</details>

As per coding guidelines, `**/*_test.go`: Use `t.Parallel()` for independent subtests.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestCreateACPClientUsesPerJobRuntimeWhenPresent(t *testing.T) {
	var captured agent.ClientConfig
	restore := SwapNewAgentClientForTest(func(_ context.Context, cfg agent.ClientConfig) (agent.Client, error) {
		captured = cfg
		return &capturingCommandIOClient{}, nil
	})
	defer restore()
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/internal/acpshared/command_io_test.go` around lines 186 -
194, The test TestCreateACPClientUsesPerJobRuntimeWhenPresent must not call
t.Parallel() because SwapNewAgentClientForTest mutates package-global
constructor state; remove the t.Parallel() invocation from that test (or convert
the test into a non-parallel setup section that calls SwapNewAgentClientForTest
and then runs parallel subtests with t.Run + t.Parallel() if needed) so that the
global factory swap performed by SwapNewAgentClientForTest is not observed by
other parallel tests.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:7bb75d9d-dbd3-41c6-89da-03415801a6e9 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `TestCreateACPClientUsesPerJobRuntimeWhenPresent` calls `SwapNewAgentClientForTest`, which swaps package-global factory state, and still marks the whole test `t.Parallel()`.
  - Root cause: the test advertises itself as independent even though it mutates global constructor state shared by other ACP client tests.
  - Intended fix: make this test non-parallel so the factory swap cannot leak across concurrently executing tests.
  - Resolution: the ACP client factory-swap test no longer runs in parallel.
