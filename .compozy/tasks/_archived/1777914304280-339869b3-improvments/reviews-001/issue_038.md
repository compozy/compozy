---
status: resolved
file: internal/daemon/workspace_refresh.go
line: 188
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUl1,comment:PRRC_kwDORy7nkc68K-Q5
---

# Issue 038: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Treat non-ENOENT `os.Stat` failures as unavailable workspaces too.**

When `os.Stat` returns permission denied or another filesystem error, this helper only sets `Warning`. The refresh loop never enters the `Missing` branch, so the workspace keeps its old filesystem state and remains selectable even though later calls will reject it as unavailable.


<details>
<summary>Suggested fix</summary>

```diff
 	return workspacePathState{
+		Missing: true,
 		Warning: fmt.Sprintf("%s: workspace path check failed: %v", rootDir, err),
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/workspace_refresh.go` around lines 170 - 188, The helper
currently treats only os.ErrNotExist as Missing and leaves other os.Stat errors
as just Warning; change it so any non-nil error from os.Stat (other than a
successful stat) causes the workspace to be marked Missing:true and include the
error in the Warning. Update the branch after the initial os.Stat call
(referencing rootDir, info, err, and workspacePathState) so that on err != nil
(and not just errors.Is(err, os.ErrNotExist)) you return
workspacePathState{Missing: true, Warning: fmt.Sprintf("%s: workspace path check
failed: %v", rootDir, err)}; keep the existing IsDir handling for the success
case.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:8d3d9d9d-d8a1-4421-95c0-379c08c617ed -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed non-ENOENT `os.Stat` failures only produced warnings, leaving unavailable workspaces selectable with stale filesystem state. Marked any stat failure as `Missing` while preserving the warning message.
