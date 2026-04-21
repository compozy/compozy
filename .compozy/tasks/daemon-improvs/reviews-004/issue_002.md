---
status: resolved
file: internal/api/core/handlers_contract_test.go
line: 214
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58kNfk,comment:PRRC_kwDORy7nkc656GMl
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Goroutine may leak if test fails before heartbeat arrives.**

If the HTTP request fails (lines 246-248) or `ReadSSEFramesUntil` times out before a heartbeat frame triggers `close(sendOverflow)`, this goroutine blocks forever on `<-sendOverflow`. Consider using a context with cancellation to ensure cleanup on test failure.



<details>
<summary>Suggested approach using context cancellation</summary>

```diff
+	ctx, cancel := context.WithCancel(context.Background())
+	defer cancel()
+
 	go func() {
 		event := events.Event{
 			SchemaVersion: events.SchemaVersion,
 			RunID:         runID,
 			Seq:           7,
 			Timestamp:     now,
 			Kind:          events.EventKindSessionUpdate,
 			Payload:       json.RawMessage(`{"delta":"hello"}`),
 		}
 		stream.events <- core.RunStreamItem{Event: &event}
-		<-sendOverflow
+		select {
+		case <-sendOverflow:
+		case <-ctx.Done():
+			close(stream.events)
+			close(stream.errors)
+			return
+		}
 		stream.events <- core.RunStreamItem{Overflow: &core.RunStreamOverflow{Reason: "slow consumer"}}
 		close(stream.events)
 		close(stream.errors)
 	}()
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		event := events.Event{
			SchemaVersion: events.SchemaVersion,
			RunID:         runID,
			Seq:           7,
			Timestamp:     now,
			Kind:          events.EventKindSessionUpdate,
			Payload:       json.RawMessage(`{"delta":"hello"}`),
		}
		stream.events <- core.RunStreamItem{Event: &event}
		select {
		case <-sendOverflow:
		case <-ctx.Done():
			close(stream.events)
			close(stream.errors)
			return
		}
		stream.events <- core.RunStreamItem{Overflow: &core.RunStreamOverflow{Reason: "slow consumer"}}
		close(stream.events)
		close(stream.errors)
	}()
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers_contract_test.go` around lines 200 - 214, The
goroutine in the test blocks on `<-sendOverflow` and can leak if the test fails
before a heartbeat; update the goroutine to respect a cancellation context or
test “done” signal: accept a ctx (or use t.Cleanup to create one) and replace
the blocking `<-sendOverflow` with a select that listens for either `case
<-sendOverflow:` or `case <-ctx.Done():` so the goroutine can exit on test
cancellation; ensure you still send the overflow event to `stream.events`
(RunStreamItem with Overflow) and close `stream.events`/`stream.errors` on
shutdown.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:64d446f5-a8b3-4889-8386-c906de9bcd0a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The producer goroutine blocks on `<-sendOverflow` after enqueueing the first event. If `client.Do` fails, or if the SSE read exits before a heartbeat closes `sendOverflow`, the goroutine never reaches its cleanup path.
  - Root cause: the test has no cancellation path that guarantees the goroutine can unwind when the test aborts early.
  - Fix approach: add a test-owned cancellation context and switch the goroutine to a `select` so it exits cleanly on either `sendOverflow` or test shutdown.
  - Resolution: the test now owns `producerCtx`/`cancelProducer`, and the producer goroutine selects between `sendOverflow` and `producerCtx.Done()` before sending the overflow frame and closing the stream channels.
  - Regression coverage: the updated `TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads` still proves the canonical event, heartbeat, and overflow payload flow while also guaranteeing producer cleanup on early test termination.
  - Verification: `go test ./internal/api/core -run 'TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads|TestDaemonHealthReturnsCanonicalEnvelopeForReadyAndDegradedStates|TestRunStartEndpointsReturnCanonicalRunEnvelopes|TestTransportErrorsUseCanonicalCodeAndRequestIDFields' -count=1` passed. `make verify` also passed with `2548` tests, `2` skipped helper-process tests, and a successful `go build ./cmd/compozy`.
