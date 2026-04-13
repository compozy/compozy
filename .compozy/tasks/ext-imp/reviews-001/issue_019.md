---
status: resolved
file: internal/core/run/exec/exec.go
line: 263
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0ig,comment:PRRC_kwDORy7nkc624f70
---

# Issue 019: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**`run.pre_start` can desynchronize `cfg` and prepared run state.**

`prepareExecRunState` runs before `applyExecRunPreStartHook`. If the hook mutates state-defining fields (e.g., `WorkspaceRoot`, `RunID`, persistence-related fields), artifacts/journal paths are already locked to old values, but record/config are updated later.

<details>
<summary>🧭 Safer guard (reject incompatible mutations after hook)</summary>

```diff
 	state, err := prepareExecRunState(ctx, cfg, scope)
 	if err != nil {
 		return "", nil, nil, job{}, err
 	}
+	originalWorkspaceRoot := cfg.WorkspaceRoot
+	originalRunID := strings.TrimSpace(cfg.RunID)
 	if err := applyExecRunPreStartHook(ctx, state, cfg); err != nil {
 		state.close()
 		return "", nil, nil, job{}, err
 	}
+	if originalRunID == "" && cfg.WorkspaceRoot != originalWorkspaceRoot {
+		state.close()
+		return "", nil, nil, job{}, fmt.Errorf("run.pre_start cannot mutate workspace_root for newly allocated exec runs")
+	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec/exec.go` around lines 244 - 263, The pre-start hook
can change cfg fields that define the prepared state (e.g., WorkspaceRoot,
RunID, persistence), so either run applyExecRunPreStartHook(ctx, cfg) before
prepareExecRunState(ctx, cfg, scope) or, if ordering cannot change, re-run or
refresh the prepared state immediately after the hook (call prepareExecRunState
again or invoke state.refreshRuntimeConfig(cfg) and any path/lock recomputation)
so artifacts/journal paths are computed from the final cfg; update the sequence
around prepareExecRunState, applyExecRunPreStartHook,
state.refreshRuntimeConfig, and any locking to ensure consistency.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:03d7857f-d529-43ca-be71-43278e85f981 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `prepareExecRunState` allocates scope-backed run artifacts and output behavior before `applyExecRunPreStartHook`, but the hook can still mutate `cfg`.
  - Root cause: state-defining fields can change after the exec run scope has already been prepared, which can desynchronize `cfg`, persisted metadata, event/output mode, and artifact paths.
  - Intended fix: allow safe config mutations to continue, but reject state-defining mutations after `run.pre_start` with an explicit error and regression coverage.
  - Resolution: added explicit guards for state-defining `run.pre_start` mutations and regression coverage for the rejected fields.
