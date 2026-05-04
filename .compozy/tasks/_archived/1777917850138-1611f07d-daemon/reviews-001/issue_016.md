---
status: resolved
file: internal/api/udsapi/server.go
line: 144
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm-,comment:PRRC_kwDORy7nkc65HKYJ
---

# Issue 016: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Mark the server as started before the listen step.**

This has the same double-start race as the HTTP transport: two callers can both pass the guard, create listeners, and then race to publish state. The later writer wins, and `Shutdown` no longer owns the first server instance.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/udsapi/server.go` around lines 139 - 144, The start routine
currently checks s.started under s.mu, unlocks, then proceeds to create
listeners allowing a race where two callers both create listeners; fix by
holding s.mu, set s.started = true before performing the listen/accept setup (so
the first caller wins), then release the lock and continue creating the
listener. Update the code around the s.mu/s.started checks in server.start (or
the method containing the shown snippet) to set s.started = true while still
holding s.mu, and only then unlock—keeping the same error return path if already
started.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4748c55f-38a6-4940-81c9-cabca13fdd92 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `udsapi.Server.Start` publishes `s.started` only after listen/setup finishes, leaving the same double-start race window as the HTTP transport.
- Fix plan: reserve the started state under lock, rollback on startup failures before serving, and add regression coverage around the reserved-state / retry path. A fully deterministic concurrent reproducer may require using the strongest seam available in the existing transport tests.
- Resolution: Implemented and verified with `make verify`.
