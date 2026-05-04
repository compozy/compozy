---
status: resolved
file: web/src/systems/app-shell/components/app-shell-container.tsx
line: 67
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUmC,comment:PRRC_kwDORy7nkc68K-RK
---

# Issue 047: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Resolve stale-workspace errors against the failing operation, not the current refs.**

These cache notifications can land after a workspace switch. At that point `selectedWorkspaceIdRef` / `activeWorkspaceIdRef` may already point at the new workspace, so a late stale error from the old workspace will clear the wrong selection and force-open the picker for the wrong id. Please derive the stale workspace from the failing query/mutation key or error payload before calling `clearActiveWorkspaceSelection()`.



Also applies to: 70-83

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/src/systems/app-shell/components/app-shell-container.tsx` around lines 60
- 67, The handler handlePossibleStaleWorkspace currently reads
selectedWorkspaceIdRef/activeWorkspaceIdRef which can have advanced by the time
a stale-workspace error arrives; instead derive the stale workspace id from the
failing operation (e.g., the query/mutation key or an id embedded in the error
payload) and use that derived id when calling setStaleSignal,
clearActiveWorkspaceSelection, and setShowPicker; update the call sites that
invoke handlePossibleStaleWorkspace (and the duplicate logic around the same
block) to pass the failing operation/error object so the handler can extract the
correct workspace id rather than reading the live refs
selectedWorkspaceIdRef/activeWorkspaceIdRef.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e59768e0-9289-4ad8-8c6f-2ed95ddf0cc4 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: The stale-workspace handler used current refs when a cache error arrived, which can misattribute an old workspace failure after the user switches workspaces. The fix derives the workspace id from the failing query/mutation key or variables and only clears selection when that id still matches the active or selected workspace.
