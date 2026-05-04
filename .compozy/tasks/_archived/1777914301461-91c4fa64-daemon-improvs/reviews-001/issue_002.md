---
status: resolved
file: internal/api/client/client_contract_test.go
line: 253
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go7Z,comment:PRRC_kwDORy7nkc651ULw
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Potential race condition when modifying package-level variable.**

Modifying `streamHeartbeatGapTolerance` at package level could cause race conditions if other tests run concurrently and depend on this value. While `t.Cleanup` restores it, the test itself is marked `t.Parallel()`.

Consider using a test-specific configuration injection mechanism or running this specific test without `t.Parallel()` to avoid flakiness.


<details>
<summary>🛡️ Suggested fix: Remove t.Parallel() for this test</summary>

```diff
 func TestOpenRunStreamReconnectsFromLastAcknowledgedCursorAfterHeartbeatGap(t *testing.T) {
-	t.Parallel()
-
 	previousGap := streamHeartbeatGapTolerance
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/client/client_contract_test.go` around lines 249 - 253, This
test mutates the package-level variable streamHeartbeatGapTolerance while also
using t.Parallel(), risking races; remove t.Parallel() from the test (or
alternatively change the test to inject a per-test configuration instead of
modifying streamHeartbeatGapTolerance) and keep the existing t.Cleanup that
restores the previousGap to ensure isolation; locate the mutation of
streamHeartbeatGapTolerance and the t.Parallel() call in the test and delete the
parallelization (or refactor to accept a test-specific timeout/config parameter
rather than touching the package variable).
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:1ff09c53-75d5-4087-95c2-121a68197733 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestOpenRunStreamReconnectsFromLastAcknowledgedCursorAfterHeartbeatGap` mutates the package-level `streamHeartbeatGapTolerance` override while also calling `t.Parallel()`, so sibling client-package tests can observe the shortened timeout and race on shared state.
- Fix approach: keep the existing override coverage but serialize this one test and continue restoring the package variable through `t.Cleanup`.
- Resolution: removed `t.Parallel()` from the heartbeat-gap reconnect test and kept the cleanup-based restoration of `streamHeartbeatGapTolerance`.
- Regression coverage: `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed after the change.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
