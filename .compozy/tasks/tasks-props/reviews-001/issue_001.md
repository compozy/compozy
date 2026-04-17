---
status: resolved
file: internal/cli/form.go
line: 42
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypyt,comment:PRRC_kwDORy7nkc644MsL
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't make this confirm disable existing task-runtime rules.**

`defineTaskRuntime` is derived from `state.taskRuntimeRules()`, but “No” here clears `executionTaskRuntimeRules` and flips `replaceConfiguredTaskRunRules`. That means a start flow with workspace-configured rules or explicit `--task-runtime` flags can silently lose those overrides just because the user declined the extra form step. This needs to distinguish “edit/add per-task rules” from “keep using the rules already in effect”.



Also applies to: 98-98, 120-128, 170-170

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form.go` around lines 35 - 42, The current branch flips
state.replaceConfiguredTaskRunRules and clears state.executionTaskRuntimeRules
whenever inputs.defineTaskRuntime is false, which causes existing
workspace-configured rules or explicit --task-runtime flags to be lost if the
user declines the extra form; change the logic in the start-flow handlers (the
block using state.kind == commandKindStart, the collectStartTaskRuntimeForm
call, and the other similar spots) so that you only mutate
state.replaceConfiguredTaskRunRules or clear state.executionTaskRuntimeRules
when the user explicitly chooses to define/replace per-task rules (i.e., when
inputs.defineTaskRuntime is true and/or when collectStartTaskRuntimeForm returns
a positive confirmation), and do not change or clear those fields when the user
declines the form; apply the same fix to the other occurrences mentioned (the
blocks around state.taskRuntimeRules() usages).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:88ec5306-112e-4d23-8001-452c7308ec4a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `collectFormParams` clears `executionTaskRuntimeRules` and `formInputs.apply` flips `replaceConfiguredTaskRunRules` when the user declines the per-task runtime form.
  - Root cause: the confirm field currently treats "do not open the extra editor" as "replace existing task-runtime rules with none", which drops workspace-configured and CLI-provided rules.
  - Intended fix: preserve the currently effective rules unless the user actually opens and submits the per-task runtime editor, and only mark config replacement when new execution rules are explicitly authored.
  - Resolution: the start-flow confirm now preserves the active task-runtime rules when the user skips the extra form, and a CLI regression test covers that preservation path.
