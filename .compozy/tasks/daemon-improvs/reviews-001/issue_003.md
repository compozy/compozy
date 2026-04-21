---
status: resolved
file: internal/api/client/client_transport_test.go
line: 213
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go7i,comment:PRRC_kwDORy7nkc651UL6
---

# Issue 003: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Assert how `force` is encoded for `StopDaemon`.**

This stub accepts both stop calls based only on method + path, so the test still passes if `StopDaemon(true)` silently drops the `force` flag. Please validate the expected query/body shape for the second request so this actually guards the transport contract.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/client/client_transport_test.go` around lines 201 - 213, The
test stub for the stop endpoint is too permissive and doesn't assert the
StopDaemon(force bool) encoding; update the Transport roundTripFunc handler for
the case handling "POST /api/daemon/stop" to validate the request shape (either
check req.URL.Query() for a force=true param or decode the JSON body and assert
{"force":true} as appropriate for StopDaemon), and only return
jsonResponse(http.StatusAccepted, `{"accepted":true}`) when that shape matches;
ensure the test fails if StopDaemon(true) omits or mis-encodes the force flag so
the transport contract for StopDaemon is actually enforced.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5db983a7-a2cf-4801-8b6f-2aa2e3a5e394 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the stop-endpoint transport stub only matches `POST /api/daemon/stop`, so the test does not fail if `StopDaemon(true)` drops or mis-encodes the `force` flag.
- Fix approach: tighten the stub so the first stop call asserts no `force` query and the second asserts `force=true`, making the test guard the real wire encoding from `internal/api/client/operator.go`.
- Resolution: the transport stub now counts stop calls and explicitly checks that `StopDaemon(false)` sends no `force` query while `StopDaemon(true)` sends `force=true`.
- Regression coverage: `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed after the change.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
