---
status: resolved
file: internal/core/sync.go
line: 65
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUle,comment:PRRC_kwDORy7nkc68K-Qb
---

# Issue 026: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Reject mismatched workspace/target pairs in `SyncWithDB`.**

This entry point trusts `workspace.ID` but resolves the filesystem target independently from `cfg`. A caller can pass workspace A together with `WorkspaceRoot`/`TasksDir` from workspace B and end up writing B’s artifacts into A’s catalog rows.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/sync.go` around lines 45 - 65, SyncWithDB currently trusts
workspace.ID but resolves the filesystem target independently (via
resolveSyncTarget), allowing callers to pass a Workspace A with cfg pointing at
Workspace B's roots; fix by validating the resolved target against the provided
workspace: after target, singleWorkflow := resolveSyncTarget(cfg) return, check
that the target's workspace identifier (e.g., any WorkspaceID/Root-derived
identity or the paths in WorkspaceRoot/TasksDir used by resolveSyncTarget)
matches strings.TrimSpace(workspace.ID) and if not, return an error (e.g.,
"mismatched workspace and sync target"); alternatively, make resolveSyncTarget
accept the workspace (or its ID) so the target is derived from the provided
workspace and reject/produce an error when they differ before calling
syncResolvedTarget.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:8d3d9d9d-d8a1-4421-95c0-379c08c617ed -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed `SyncWithDB` accepted an already-resolved workspace row but independently resolved `cfg.TasksDir`, allowing artifacts from another workspace to be reconciled under the provided workspace ID. Added workspace-root containment validation before syncing and covered the mismatch with a regression test.
