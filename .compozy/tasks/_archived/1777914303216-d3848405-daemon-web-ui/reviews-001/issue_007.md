---
status: resolved
file: internal/api/httpapi/browser_middleware.go
line: 78
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqXH,comment:PRRC_kwDORy7nkc651WG0
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Require the active-workspace header on workspace-scoped browser routes.**

If the header is absent, this middleware falls through and the handlers still accept `workspace` from query/body. That makes the browser-only workspace context optional and lets callers bypass the stale/missing-context checks those routes are supposed to enforce.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/browser_middleware.go` around lines 74 - 78, The
middleware currently lets requests without the active-workspace header fall
through (workspaceID :=
strings.TrimSpace(c.GetHeader(core.HeaderActiveWorkspaceID)) ... if workspaceID
== "" { c.Next(); return }), which allows handlers to accept workspace from
query/body and bypass workspace-scoped checks; change this to enforce the header
by aborting the request when workspaceID is empty (e.g., call
c.AbortWithStatusJSON or similar on the Gin context `c` with a clear error
message and status like 400/401) instead of calling c.Next(), so handlers cannot
proceed without core.HeaderActiveWorkspaceID present.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e75280e4-7172-485f-934b-c3510e24ebf0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Workspace-scoped browser routes currently allow missing headers to continue, which lets handlers fall back to query/body workspace refs and bypass the intended browser context guard.
  - Root cause: `activeWorkspaceMiddleware` treats an empty `X-Compozy-Workspace-ID` header as optional even on routes that explicitly require an active workspace.
  - Intended fix: reject missing headers on workspace-scoped browser routes before handlers run, then update affected HTTP integration expectations.

## Resolution

- Updated the browser workspace middleware to reject missing `X-Compozy-Workspace-ID` headers on workspace-scoped routes with `412 workspace_context_missing` and aligned the HTTP integration coverage with that behavior.
- Verified with `make verify`.
