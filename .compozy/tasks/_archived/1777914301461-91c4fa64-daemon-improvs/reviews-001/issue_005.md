---
status: resolved
file: internal/api/contract/contract_integration_test.go
line: 230
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go7s,comment:PRRC_kwDORy7nkc651UME
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Guard `sendOverflow` against repeated heartbeats.**

This closes `sendOverflow` on every heartbeat. If the stream emits a second heartbeat before the overflow frame is observed, the subtest will panic with `close of closed channel`.  


<details>
<summary>Suggested fix</summary>

```diff
+	var overflowOnce sync.Once
 	for heartbeat == nil || overflow == nil {
 		select {
 		case frame, ok := <-framesCh:
 			if !ok {
 				if err := <-errCh; err != nil {
@@
 			switch frame.Event {
 			case "heartbeat":
 				var payload contract.HeartbeatPayload
 				if err := json.Unmarshal(frame.Data, &payload); err != nil {
 					t.Fatalf("decode heartbeat payload: %v", err)
 				}
 				heartbeat = &payload
-				close(sendOverflow)
+				overflowOnce.Do(func() { close(sendOverflow) })
 			case "overflow":
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/contract/contract_integration_test.go` around lines 207 - 230,
In the loop handling framesCh, avoid closing sendOverflow on every heartbeat;
change the heartbeat branch (where you currently call close(sendOverflow)) to
only close it once by checking if sendOverflow != nil before closing and then
set sendOverflow = nil (or use a boolean flag) so subsequent heartbeat events
won't attempt to close an already-closed channel; update the code around the
heartbeat handling in the test (where heartbeat, overflow, framesCh, and
sendOverflow are referenced) accordingly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5db983a7-a2cf-4801-8b6f-2aa2e3a5e394 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the heartbeat branch closes `sendOverflow` unconditionally. If the stream emits another heartbeat before the overflow frame arrives, the test panics with `close of closed channel`.
- Fix approach: guard the close with once semantics so repeated heartbeats remain safe while preserving the current overflow sequencing behavior.
- Resolution: the integration test now requests overflow only once via an `overflowRequested` guard while still decoding the first heartbeat and overflow payloads.
- Regression coverage: `go test -tags integration ./internal/api/contract` passed after the fix.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
