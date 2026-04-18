---
status: resolved
file: internal/api/core/handlers_test.go
line: 115
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm0,comment:PRRC_kwDORy7nkc65HKX_
---

# Issue 009: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Replace the sleep-based stream sequencing.**

This assertion depends on `time.Sleep(30 * time.Millisecond)` being long enough for a heartbeat to win the race before the overflow frame is sent. That will be flaky under slower CI or `-race`. Please gate the overflow emission on an explicit synchronization point instead of wall-clock delay.


As per coding guidelines, "NEVER use `time.Sleep()` in orchestration — use proper synchronization primitives instead".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers_test.go` around lines 107 - 115, The test
currently uses time.Sleep inside the goroutine that sends a
core.RunStreamOverflow to stream.events, which races with the heartbeat; replace
the sleep with an explicit synchronization primitive: add a channel or
sync.WaitGroup (e.g., readyCh) that the main test signals once the heartbeat has
been processed, then have the goroutine block on that signal before sending the
overflow frame to stream.events and closing stream.events/stream.errors; update
the test to wait for the ready signal rather than sleeping so newFakeRunStream,
stream.events, and the overflow send are deterministically ordered.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c36ff8f6-d8b2-44c7-a1cd-4e6f4795be06 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: the heartbeat/overflow ordering in `TestStreamRunEmitsHeartbeatAndOverflowFrames` relies on `time.Sleep`, which makes the assertion timing-dependent and vulnerable to slow CI / `-race`.
- Fix plan: replace the sleep with an explicit synchronization signal so the overflow event is emitted only after the first heartbeat is observed.
- Resolution: Implemented and verified with `make verify`.
