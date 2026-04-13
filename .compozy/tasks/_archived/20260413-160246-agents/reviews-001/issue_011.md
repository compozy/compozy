---
status: resolved
file: internal/core/plan/prepare_test.go
line: 389
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tz,comment:PRRC_kwDORy7nkc62zc8j
---

# Issue 011: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Set `XDG_CONFIG_HOME` here too.**

This test only overrides `HOME`. If the runner already has `XDG_CONFIG_HOME` set, `ResolveExecutionContext` can still discover real global agents and make the catalog assertions flaky. Point `XDG_CONFIG_HOME` at a temp dir alongside `HOME`.


<details>
<summary>Suggested test hardening</summary>

```diff
 	workspaceRoot := t.TempDir()
 	homeDir := t.TempDir()
 	t.Setenv("HOME", homeDir)
+	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
```
</details>

Based on learnings: Find and document edge cases that the happy path ignores.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/plan/prepare_test.go` around lines 387 - 389, The test
currently sets workspaceRoot and homeDir but only overrides HOME, so
ResolveExecutionContext may still read a pre-existing XDG_CONFIG_HOME; create a
separate temp dir (e.g., xdgConfigDir := t.TempDir()) and call
t.Setenv("XDG_CONFIG_HOME", xdgConfigDir) alongside t.Setenv("HOME", homeDir)
(references: workspaceRoot, homeDir) to isolate config discovery and prevent
flaky catalog assertions.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:44db1207-e0c3-4af8-b043-4fce2c12b432 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The selected-agent prepare test overrode `HOME` but not `XDG_CONFIG_HOME`, so real global agent configuration could still leak into discovery.
- Fix: Added a temp `XDG_CONFIG_HOME` alongside the temp home directory in the affected test.
- Evidence: `go test ./internal/core/plan`
