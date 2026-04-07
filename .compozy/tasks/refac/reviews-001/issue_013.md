---
status: resolved
file: internal/core/migration/workflow_target.go
line: 31
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWa,comment:PRRC_kwDORy7nkc61XmRB
---

# Issue 013: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**Reject workflow names that escape `rootDir`.**

`cfg.name` is joined onto `rootDir` verbatim. Values like `../other`, `foo/bar`, or an absolute path escape the workflow root, and the archive flow can then rename that arbitrary directory into `_archived`. Treat `name` as a single workflow directory component and fail fast if it contains separators or dot segments.  


<details>
<summary>🔒 Suggested hardening</summary>

```diff
 func resolveWorkflowTarget(cfg workflowTargetOptions) (workflowTargetResolution, error) {
+	workflowName := strings.TrimSpace(cfg.name)
+	if workflowName != "" {
+		if workflowName != filepath.Base(workflowName) || !model.IsActiveWorkflowDirName(workflowName) {
+			return workflowTargetResolution{}, fmt.Errorf(
+				"%s name must be a single active workflow directory name",
+				cfg.command,
+			)
+		}
+	}
+
 	specificTargets := 0
-	if strings.TrimSpace(cfg.name) != "" {
+	if workflowName != "" {
 		specificTargets++
 	}
 	if strings.TrimSpace(cfg.tasksDir) != "" {
 		specificTargets++
 	}
@@
-	case strings.TrimSpace(cfg.name) != "":
-		target = filepath.Join(rootDir, strings.TrimSpace(cfg.name))
+	case workflowName != "":
+		target = filepath.Join(rootDir, workflowName)
 		specificTarget = true
 	}
```
</details>


Also applies to: 61-63

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/workflow_target.go` around lines 30 - 31, Reject
workflow names that can escape the workflow root by validating cfg.name before
joining it to rootDir: ensure cfg.name is non-empty and does not contain path
separators ('/' or '\\'), dot-segments ('.' or '..'), or an absolute path
prefix; if any of these are present return an error (fail fast). Update the
check around strings.TrimSpace(cfg.name) != "" (and the similar check at the
other occurrence around lines 61-63) to perform this validation and use the same
validation helper wherever cfg.name is used to form a path to prevent directory
traversal/escape.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d2033f55-fd4d-4209-b796-3f58621d2a7d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `resolveWorkflowTarget` joins `cfg.name` directly under `rootDir`, so path separators, dot segments, and absolute paths can escape the workflow root. That creates a real traversal risk for migration/archive flows. The fix is to validate workflow names as single active-directory components before they are joined into a filesystem path.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
