---
status: resolved
file: internal/cli/commands_simple.go
line: 322
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mnA,comment:PRRC_kwDORy7nkc65HKYL
---

# Issue 019: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Silently ignoring `absoluteWorkflowPath` error may mask configuration issues.**

When `--root-dir` is provided but `absoluteWorkflowPath` fails, the error is silently ignored and `archiveRootBase` falls back to the default. This could mask user configuration errors like invalid paths.

Consider logging a warning or returning the error to help users diagnose misconfigured `--root-dir` values.

<details>
<summary>🛠️ Proposed fix to propagate the error</summary>

```diff
 	archiveRootBase := model.TasksBaseDirForWorkspace(s.workspaceRoot)
 	if strings.TrimSpace(s.rootDir) != "" {
 		resolvedRoot, err := absoluteWorkflowPath(s.workspaceRoot, s.rootDir)
-		if err == nil {
-			archiveRootBase = resolvedRoot
+		if err != nil {
+			return nil, fmt.Errorf("resolve archive root: %w", err)
 		}
+		archiveRootBase = resolvedRoot
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	archiveRootBase := model.TasksBaseDirForWorkspace(s.workspaceRoot)
	if strings.TrimSpace(s.rootDir) != "" {
		resolvedRoot, err := absoluteWorkflowPath(s.workspaceRoot, s.rootDir)
		if err != nil {
			return nil, fmt.Errorf("resolve archive root: %w", err)
		}
		archiveRootBase = resolvedRoot
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/commands_simple.go` around lines 316 - 322, The current logic
silently drops errors from absoluteWorkflowPath when s.rootDir is provided,
which can hide misconfiguration; update the block that sets archiveRootBase
(using model.TasksBaseDirForWorkspace, s.workspaceRoot, s.rootDir) to surface
failures from absoluteWorkflowPath: either return the error up to the caller or
log a clear warning including the failed s.rootDir and the error message so
users see invalid path problems instead of silently falling back to the default;
ensure you modify the surrounding function signature/return path if you choose
to propagate the error.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:cf0bde9c-2082-49b9-84b0-62c22ed0cd58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `archiveViaDaemon` silently discards `absoluteWorkflowPath` failures when `--root-dir` is supplied, which can hide a bad root-path configuration and fall back to the default archive base unexpectedly.
- Fix plan: surface the resolution error instead of silently falling back and add/adjust coverage for the error path.
- Resolution: Implemented and verified with `make verify`.
