---
status: resolved
file: internal/cli/workspace_config_test.go
line: 279
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzE,comment:PRRC_kwDORy7nkc644Msp
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Make this test hermetic to global config.**

`applyWorkspaceDefaults()` still consults the global config path, but this case never isolates `HOME`. A developer or CI environment with `~/.compozy/config.toml` and extra `start.task_runtime_rules` will change `state.configuredTaskRuntimeRules` and break the `len==1` / ordering assertions here. Please isolate the config home for this test, or stop running it in parallel so the existing helper can do that safely.

<details>
<summary>Suggested fix</summary>

```diff
 func TestApplyWorkspaceDefaultsKeepsConfiguredTaskRuntimeRulesAndBuildConfigAppendsCLIOverrides(t *testing.T) {
-	t.Parallel()
+	isolateCLIConfigHome(t)

 	root := t.TempDir()
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestApplyWorkspaceDefaultsKeepsConfiguredTaskRuntimeRulesAndBuildConfigAppendsCLIOverrides(t *testing.T) {
	isolateCLIConfigHome(t)

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[start]
[[start.task_runtime_rules]]
type = "frontend"
ide = "claude"
model = "sonnet"
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Var(
		newTaskRuntimeFlagValue(&state.executionTaskRuntimeRules),
		"task-runtime",
		"task runtime",
	)

	if err := cmd.Flags().Set("task-runtime", "id=task_01,model=gpt-5.4-mini"); err != nil {
		t.Fatalf("set task-runtime flag: %v", err)
	}

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if len(state.configuredTaskRuntimeRules) != 1 {
		t.Fatalf("unexpected configured task runtime rules: %#v", state.configuredTaskRuntimeRules)
	}

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.TaskRuntimeRules) != 2 {
		t.Fatalf("unexpected merged task runtime rules: %#v", cfg.TaskRuntimeRules)
	}
	if cfg.TaskRuntimeRules[0].Type == nil || *cfg.TaskRuntimeRules[0].Type != "frontend" {
		t.Fatalf("expected config type rule first, got %#v", cfg.TaskRuntimeRules[0])
	}
	if cfg.TaskRuntimeRules[1].ID == nil || *cfg.TaskRuntimeRules[1].ID != "task_01" {
		t.Fatalf("expected CLI id rule to append after config, got %#v", cfg.TaskRuntimeRules[1])
	}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/workspace_config_test.go` around lines 229 - 279, The test
TestApplyWorkspaceDefaultsKeepsConfiguredTaskRuntimeRulesAndBuildConfigAppendsCLIOverrides
is not hermetic because applyWorkspaceDefaults reads the global config from
HOME; make the test isolated by setting HOME to a temp directory before calling
state.applyWorkspaceDefaults (or use t.Setenv("HOME", tmpDir) if available) and
restore/cleanup after, or alternatively remove t.Parallel() to avoid concurrent
interference; ensure you perform this HOME isolation before calling
writeCLIWorkspaceConfig/chdirCLITest and before state.applyWorkspaceDefaults so
configuredTaskRuntimeRules only reflects the test fixtures.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:88ec5306-112e-4d23-8001-452c7308ec4a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `TestApplyWorkspaceDefaultsKeepsConfiguredTaskRuntimeRulesAndBuildConfigAppendsCLIOverrides` calls `applyWorkspaceDefaults`, which reads merged workspace and HOME config, but the test never isolates HOME.
  - Root cause: the test assumes only its workspace fixture contributes runtime rules, even though a developer or CI global config can inject additional defaults.
  - Intended fix: isolate CLI HOME before discovering config and drop `t.Parallel()` for this case because HOME mutation is process-global.
  - Resolution: the CLI workspace-config test now isolates HOME before loading config and no longer runs in parallel.
