---
status: resolved
file: internal/core/extension/manager_active.go
line: 58
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iS,comment:PRRC_kwDORy7nkc624f7l
---

# Issue 010: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Reject ambiguous cross-run session reuse.**

`lookupActiveExtensionSession` picks the first matching session from a process-wide map keyed only by workspace and extension name. If two runs in the same workspace have the same extension active, map iteration order makes this nondeterministic, so the bridge can attach to another run’s session and leak event/audit/shutdown coupling across run boundaries. Please scope the lookup by explicit owner (for example run/manager identity) or fail closed when more than one candidate exists.


Based on learnings: "Maintain clear system boundaries and establish clear ownership of each boundary."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/manager_active.go` around lines 35 - 58,
lookupActiveExtensionSession currently returns the first session matching
workspaceRoot and extensionName from activeManagers, risking cross-run reuse;
update the lookup to require explicit owner (run/manager identity) or, if owner
is not provided, detect when more than one candidate exists and fail closed by
returning nil. Specifically, modify lookupActiveExtensionSession to accept an
owner identifier (or obtain manager identity) and when iterating
activeManagers.managers use manager identity (e.g., manager.ID or manager.owner)
alongside workspaceRoot and normalizedName to match only that manager’s session
via manager.sessionForExtension; if you cannot match a unique manager or you
detect multiple matching sessions for the workspace+extension, return nil
instead of returning the first session to avoid nondeterministic cross-run
attachment.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c30debaf-c506-49cf-8095-ed74f878d9da -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: `lookupActiveExtensionSession` iterates a process-wide manager set and returns the first workspace/name match, so two active runs in the same workspace can nondeterministically reuse the wrong extension session.
  - Fix plan: fail closed when the workspace/name lookup is ambiguous and add regression coverage so cross-run session reuse only happens when the match is unique.
  - Resolved: `internal/core/extension/manager_active.go` now returns `nil` for ambiguous matches, with regression coverage in `internal/core/extension/review_provider_bridge_integration_test.go`; verified with `make verify`.
