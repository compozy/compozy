---
status: resolved
file: internal/api/client/runs.go
line: 458
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mmw,comment:PRRC_kwDORy7nkc65HKX6
---

# Issue 004: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't convert normal backpressure into a terminal stream error.**

Once `items` fills up, `sendItem` returns `"client run stream buffer is full"`, `read()` exits, and the SSE session dies even though the server is still healthy. A burst of events or a briefly slow consumer is enough to tear down the stream.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/client/runs.go` around lines 452 - 458, sendItem currently
treats a full s.items channel as a terminal error which kills the SSE session;
instead, make sendItem wait for space and only return an error when the stream
is actually canceled. Replace the non-blocking send in sendItem with a blocking
select that sends item to s.items or returns only if the stream context/channel
is closed (e.g. check s.ctx.Done() or the stream's close channel), so normal
backpressure blocks the sender rather than terminating the stream; keep
references to sendItem, s.items and the read() consumer so the change preserves
expected lifetime semantics.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4748c55f-38a6-4940-81c9-cabca13fdd92 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `clientRunStream.sendItem` uses a non-blocking send and treats a full local buffer as a terminal stream failure, so transient consumer backpressure kills a healthy SSE session.
- Fix plan: make `sendItem` block until buffer space is available or the stream context is canceled, and add a regression test that proves backpressure no longer turns into a fatal stream error.
- Resolution: Implemented and verified with `make verify`.
