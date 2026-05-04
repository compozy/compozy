---
status: resolved
file: internal/api/core/middleware.go
line: 51
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqW6,comment:PRRC_kwDORy7nkc651WGh
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify whether callers ever pass nil to WithActiveWorkspaceID.
# If none do, the nil fallback can be removed safely.
rg -nP --type=go -C2 '\bWithActiveWorkspaceID\s*\('
rg -nP --type=go '\bWithActiveWorkspaceID\s*\(\s*nil\s*,'
```

Repository: compozy/compozy

Length of output: 813

---

🏁 Script executed:

```shell
rg -nP --type=go 'WithActiveWorkspaceID' --count
rg -nP --type=go 'WithActiveWorkspaceID'
```

Repository: compozy/compozy

Length of output: 530

---



</details>

**Remove the unnecessary nil fallback context; callers always provide a valid request-scoped context.**

The coding guidelines explicitly prohibit `context.Background()` outside `main` and focused tests. This middleware function should not include the defensive nil check at lines 49-51 since the only caller (`internal/api/httpapi/browser_middleware.go:107`) always passes a valid request context via `c.Request.Context()`. Remove the nil check entirely and require a non-nil context parameter.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/middleware.go` around lines 49 - 51, Remove the defensive
nil-context fallback in the middleware by deleting the ctx == nil check and the
context.Background() assignment so the middleware function (the context
parameter/variable `ctx` in internal/api/core/middleware.go) requires and uses a
non-nil request-scoped context; update any function signature or callers only if
necessary (note: browser_middleware.go uses c.Request.Context()), and ensure no
code paths create a background context inside this module.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:5f892913-9f2f-40c7-a5be-713f1cf7190d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The middleware helpers currently create `context.Background()` inside transport code even though the active-workspace path is always called with a request-scoped context.
  - Root cause: defensive nil-context fallbacks were left in place inside context propagation helpers instead of requiring a real caller context.
  - Intended fix: remove the background fallback on the active-workspace/request-id propagation helpers and rely on non-nil request contexts from actual callers.

## Resolution

- Removed the `context.Background()` fallback from the request-id and active-workspace middleware helpers so they preserve the real request-scoped context.
- Verified with `make verify`.
