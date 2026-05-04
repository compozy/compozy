---
status: resolved
file: internal/api/httpapi/server.go
line: 214
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm7,comment:PRRC_kwDORy7nkc65HKYG
---

# Issue 013: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**This shared handler mutation breaks dual-transport shutdown.**

`core.Handlers` only has one `streamDone` slot, and `internal/api/udsapi/server.go` writes the same field during its own `Start`. If HTTP and UDS share one handler instance, the last server to start owns stream cancellation for both transports, so shutting one transport down can leave the other transport's streams orphaned or cancel the wrong ones.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/server.go` around lines 212 - 214, The shared mutation
of core.Handlers via s.handlers.SetStreamDone in the HTTP server Start is unsafe
because internal/api/udsapi/server.go also writes the same streamDone slot;
change Start so it does not mutate a shared handlers instance. Fix by giving
each transport its own handler instance or by adding transport-scoped stream
cancellation (e.g., create a clone/copy of core.Handlers for HTTP before calling
SetStreamDone, or add a per-transport SetStreamDone variant such as
handlers.ForTransport("http").SetStreamDone), and leave SetHTTPPort usage as-is;
update the HTTP server Start (where s.mu.Lock() and s.handlers.SetStreamDone are
called) and the UDS Start in internal/api/udsapi/server.go to use
transport-specific handler instances or APIs instead of writing the same shared
slot.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4748c55f-38a6-4940-81c9-cabca13fdd92 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: HTTP and UDS startup both write a single shared `Handlers.streamDone` slot, so the most recently started transport owns stream cancellation for both servers.
- Fix plan: move stream-shutdown propagation to transport-scoped request state instead of shared mutable handler state, update both transports to use that path, and add a dual-transport regression test. The fix will stay within the listed transport/core files.
- Resolution: Implemented and verified with `make verify`.
