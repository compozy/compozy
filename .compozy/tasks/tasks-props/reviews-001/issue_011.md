---
status: resolved
file: internal/core/workspace/config_test.go
line: 394
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzV,comment:PRRC_kwDORy7nkc644Ms-
---

# Issue 011: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**These `LoadConfig` tests still depend on the caller’s real global config.**

`LoadConfig` merges workspace and global config, so these two tests can pick up a developer’s `~/.compozy/config.toml` and fail with extra rules or unrelated validation errors. They should isolate HOME (or stub `osUserHomeDir`) before loading config; that also means dropping `t.Parallel()` if you use `isolateWorkspaceConfigHome(t)`.

<details>
<summary>Suggested fix</summary>

```diff
 func TestLoadConfigParsesStartTaskRuntimeRules(t *testing.T) {
-	t.Parallel()
+	isolateWorkspaceConfigHome(t)

 	root := t.TempDir()
 	writeWorkspaceConfig(t, root, `
 ...
 func TestLoadConfigRejectsUnsupportedStartTaskRuntimeRuleID(t *testing.T) {
-	t.Parallel()
+	isolateWorkspaceConfigHome(t)

 	root := t.TempDir()
 	writeWorkspaceConfig(t, root, `
```
</details>



Also applies to: 434-452

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/workspace/config_test.go` around lines 361 - 394, The tests
TestLoadConfigParsesStartTaskRuntimeRules (and the similar one at 434-452) call
LoadConfig which merges global HOME config, so isolate the test from a
developer's ~/.compozy/config.toml by invoking isolateWorkspaceConfigHome(t) (or
stubbing osUserHomeDir) before calling LoadConfig, and remove t.Parallel() from
the test since isolateWorkspaceConfigHome requires non-parallel execution;
update both test functions to call isolateWorkspaceConfigHome(t) at the start
(or set a temporary HOME via osUserHomeDir stub) and then call LoadConfig as
before.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:7bb75d9d-dbd3-41c6-89da-03415801a6e9 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The two `LoadConfig` tests invoke HOME-merged config loading without isolating HOME first.
  - Root cause: those cases assume only workspace fixture data participates in the merged config, but a real `~/.compozy/config.toml` can inject extra rules or unrelated validation failures.
  - Intended fix: isolate HOME for both tests and remove `t.Parallel()` because process-wide environment mutation is not parallel-safe.
  - Resolution: both workspace config tests now isolate HOME before loading config and no longer run in parallel.
