---
status: resolved
file: internal/cli/form.go
line: 37
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlZ,comment:PRRC_kwDORy7nkc68K-QV
---

# Issue 014: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Clear existing task runtime rules when the user turns this off.**

`defineTaskRuntime` is prefilled from existing state, but this branch only handles the `true` path. If the user deselects the prompt, the old per-task runtime rules remain in `state` and the command can still run with overrides the form just appeared to disable.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form.go` around lines 35 - 37, The form logic currently only
handles the true path for inputs.defineTaskRuntime (calling
collectTaskRunRuntimeForm) so when a user deselects the prompt existing per-task
runtime rules in state persist; update the branch that checks state.kind ==
commandKindTasksRun to explicitly clear the stored per-task runtime rules on the
false path (i.e., when inputs.defineTaskRuntime is false) by removing/setting to
empty the state field that holds those overrides (the same state property
populated by collectTaskRunRuntimeForm), ensuring the command no longer runs
with stale runtime overrides.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:8334664c-1919-4ec7-be9e-a709acd8736c -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: The form only handled the enabled task-runtime path, leaving stale configured/execution runtime rules when the user disabled the prompt. Added `clearTaskRunRuntimeRules` on the false path, marked the `task-runtime` flag changed, and covered the clearing behavior in tests.
