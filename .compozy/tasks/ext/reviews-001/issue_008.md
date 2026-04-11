---
status: resolved
file: internal/cli/extension/root.go
line: 104
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVO0,comment:PRRC_kwDORy7nkc621Val
---

# Issue 008: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't hard-require a workspace for all `ext` commands.**

`resolveEnv` always calls `workspace.Discover`, so `compozy ext list`, `inspect`, `enable`, or `disable` will fail outside a workspace even though those commands also manage bundled and user-scoped extensions. Make workspace resolution best-effort here, and let only the subcommands that truly need workspace writes require it.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/root.go` around lines 85 - 104, resolveEnv currently
hard-fails by always calling resolveWorkspaceRoot, which prevents ext commands
from running outside a workspace; make workspace resolution best-effort by
attempting resolveWorkspaceRoot(ctx) and, on a non-fatal "no workspace" case,
set commandEnv.workspaceRoot to an empty string (or nil equivalent) and continue
returning the env, rather than returning an error. Keep resolveEnv's homeDir and
store creation unchanged, and move strict workspace validation into the specific
subcommands that require workspace writes (update those handlers to call
resolveWorkspaceRoot or check commandEnv.workspaceRoot and return a clear error
if missing). Use the function/struct names resolveEnv, resolveWorkspaceRoot, and
commandEnv to locate the changes.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d791b4d1-a09d-47ff-aca9-2faf6e21ecb7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `INVALID`
- Notes:
  - I could not reproduce the claimed hard failure. `defaultCommandDeps.resolveWorkspaceRoot` calls `workspace.Discover(ctx, "")`, and `workspace.Discover` returns the current directory when no `.compozy/` marker is found instead of failing.
  - Current behavior already allows `compozy ext ...` commands to run outside a configured workspace while still scanning bundled/user scope and the current directory's `.compozy/extensions` tree if present.
  - Changing `resolveEnv` to force an empty workspace root would alter existing discovery/state semantics rather than fix a real failure in the current tree.
