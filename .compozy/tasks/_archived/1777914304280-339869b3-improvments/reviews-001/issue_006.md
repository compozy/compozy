---
status: resolved
file: internal/api/core/handlers.go
line: 31
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUk8,comment:PRRC_kwDORy7nkc68K-Pt
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Loopback-only origins will break the workspace socket outside local dev.**

`websocket.Accept` will reject browser upgrades whose `Origin` is not one of these loopback hosts, so a deployed UI served from a real domain will fail to connect to `/api/workspaces/:id/ws`. This allow-list needs to come from config or server settings instead of being hard-coded to localhost.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers.go` around lines 27 - 31, The hard-coded
workspaceSocketOriginPatterns array restricts websocket.Accept to loopback
origins and will break production UI connections; instead make the origin
allow-list configurable and use that when calling websocket.Accept in the
workspace socket handler (the code that handles /api/workspaces/:id/ws). Replace
the literal workspaceSocketOriginPatterns with a value loaded from server config
or settings (falling back to a sensible default that still includes "localhost",
"127.0.0.1", "::1"), and update the websocket.Accept invocation to consult this
configured list when validating the Origin header.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:43567ed8-392f-4a75-9e7a-1958060562fd -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: The websocket origin allow-list was hard-coded to loopback hosts, which prevents deployed browser origins from connecting. Added `HandlerConfig.WorkspaceSocketOriginPatterns`, normalized configured values, preserved loopback defaults, copied the setting through `Clone`, and used the handler's configured list in `websocket.Accept`.
