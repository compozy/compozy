---
status: resolved
file: internal/api/httpapi/browser_middleware.go
line: 116
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlH,comment:PRRC_kwDORy7nkc68K-P8
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Avoid probing the filesystem on routes that don't enforce it.**

This `workspacePathUnavailable()` call runs before `requiresWorkspaceFilesystem(...)` is checked, so every active-workspace browser request now pays for an `os.Stat` on `workspace.RootDir`. On slow or broken mounts that can block otherwise read-only requests for no benefit. Gate the probe behind the route/method check instead.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/browser_middleware.go` around lines 110 - 116, The
filesystem probe workspacePathUnavailable(workspace) is being called
unconditionally causing unnecessary os.Stat on every request; change the logic
to first call requiresWorkspaceFilesystem(fullPath, c.Request.Method) and only
if that returns true then call workspacePathUnavailable(workspace), and if that
returns an error invoke core.RespondError with
core.WorkspacePathMissingProblem(workspace.ID, workspace.RootDir, err) and
c.Abort()/return as before; update the block around
requiresWorkspaceFilesystem(...) and workspacePathUnavailable(...) accordingly
so read-only/non-filesystem routes skip the stat.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:8334664c-1919-4ec7-be9e-a709acd8736c -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: `activeWorkspaceMiddleware` called `workspacePathUnavailable` before checking whether the route/method required filesystem access. Moved the stat behind `requiresWorkspaceFilesystem(fullPath, method)` so read-only/non-filesystem active-workspace routes skip the probe.
