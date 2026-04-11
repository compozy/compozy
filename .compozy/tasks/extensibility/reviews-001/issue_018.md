---
status: resolved
file: internal/core/extension/audit.go
line: 236
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPA,comment:PRRC_kwDORy7nkc621Va1
---

# Issue 018: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Potential race condition in Close when context is canceled.**

In the `Close` method, if the context is canceled (line 233-234), the goroutine at line 217-225 continues running in the background, but the caller returns immediately. This could lead to the audit logger being in an inconsistent state where `closeDone` channel exists but the file might not be fully synced/closed when a subsequent operation checks state.

Consider documenting this behavior or ensuring the goroutine completes regardless of context cancellation for data integrity.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/audit.go` around lines 181 - 236, The Close method
can return early on ctx cancellation while the background goroutine spawned to
call syncAndCloseFile(file) continues, leaving closeDone/closeErr set and the
file not fully closed; modify Close so that on ctx.Done() it still waits for the
background goroutine to finish (read from closeDone) before returning, then
wrap/return the context error (e.g., capture <-done then return
fmt.Errorf("close audit logger: %w", ctx.Err()) or return the file-close error
combined with ctx.Err. Update the select at the end of Close to ensure the
goroutine started in Close (which sets l.closeErr and closes l.closeDone) is
always awaited before returning, referencing symbols: Close, syncAndCloseFile,
closeDone, closeErr, file, path, mu.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:a0470a06-2cd1-4138-9d8b-04892d40a010 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes: `AuditLogger.Close` is explicitly documented to respect the caller's deadline/cancellation, so returning on `ctx.Done()` is intentional. Once close begins, `Record` rejects new writes and `Open` refuses to reopen until `closeDone` completes, so the logger does not become reopenable while the background sync/close is still running. Waiting unconditionally would violate the API contract that `Close` honors cancellation.
