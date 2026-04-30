---
status: resolved
file: internal/daemon/query_service.go
line: 765
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlg,comment:PRRC_kwDORy7nkc68K-Qf
---

# Issue 030: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**These missing-filesystem guards bypass the archived fallback you just resolved.**

`resolveWorkflowReadTarget` can already point `target.rootDir` at an archived workflow directory, but both guards return early purely because the live workspace state is `missing`. That makes spec and memory reads come back empty even when the archived fallback exists and is readable.



Also applies to: 856-858

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/query_service.go` around lines 763 - 765, The early-return
guard that checks target.workspace.FilesystemState ==
globaldb.WorkspaceFilesystemStateMissing should not unconditionally return nil
because resolveWorkflowReadTarget may have set target.rootDir to an archived
workflow; instead, change the logic in resolveWorkflowReadTarget (and the
similar guards at the other occurrence) to only return nil when both the live
workspace is missing AND there is no archived fallback available for
target.rootDir (i.e., attempt the archived-read path when target.rootDir points
at an archived workflow directory); update the branch around
target.workspace.FilesystemState to call the archived-fallback resolution code
used elsewhere rather than immediately returning nil.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:43567ed8-392f-4a75-9e7a-1958060562fd -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed memory/spec read guards returned empty results whenever the workspace filesystem state was missing, even when `resolveWorkflowReadTarget` had selected an archived workflow directory. Added archived-root detection so archived filesystem reads continue while truly missing live workspaces still return empty results.
