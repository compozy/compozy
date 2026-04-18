---
status: resolved
file: internal/cli/daemon_exec_test_helpers_test.go
line: 317
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mnB,comment:PRRC_kwDORy7nkc65HKYN
---

# Issue 023: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, let's check the file and see the full context
cd /repo && wc -l internal/cli/daemon_exec_test_helpers_test.go
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
# Look for the inProcessClientRunStream struct definition
rg "type inProcessClientRunStream" -A 5
```

Repository: compozy/compozy

Length of output: 453

---

🏁 Script executed:

```shell
# Search for usages of newInProcessClientRunStream
rg "newInProcessClientRunStream" -B 2 -A 2
```

Repository: compozy/compozy

Length of output: 752

---

🏁 Script executed:

```shell
# Check how Close() is called in tests
rg "\.Close\(\)" internal/cli/daemon_exec_test_helpers_test.go -B 3 -A 1
```

Repository: compozy/compozy

Length of output: 148

---

🏁 Script executed:

```shell
# Look for consumer patterns - where Items() or Errors() are consumed
rg "\.Items\(\)|\.Errors\(\)" internal/cli/ -B 2 -A 5
```

Repository: compozy/compozy

Length of output: 1844

---

🏁 Script executed:

```shell
# Check for goroutine leak detection in tests
rg "goleak|goroutine" --type go | head -20
```

Repository: compozy/compozy

Length of output: 1145

---

🏁 Script executed:

```shell
# Look at the full consumer code in reviews_exec_daemon.go
rg -A 20 "case item, ok := <-stream.Items\(\)" internal/cli/reviews_exec_daemon.go
```

Repository: compozy/compozy

Length of output: 627

---

🏁 Script executed:

```shell
# Check if there are early returns or ctx.Done() that might abandon stream reading
rg "stream.Items|stream.Errors" -B 5 -A 10 internal/cli/daemon_exec_test_helpers_test.go | head -50
```

Repository: compozy/compozy

Length of output: 324

---

🏁 Script executed:

```shell
# Look for actual test usage of newInProcessClientRunStream
fd "daemon_exec_test_helpers_test.go" -exec head -300 {} \; | tail -100
```

Repository: compozy/compozy

Length of output: 289

---

🏁 Script executed:

```shell
# Get more context about the test file and its usage
rg -B 10 -A 30 "func newInProcessClientRunStream" internal/cli/daemon_exec_test_helpers_test.go
```

Repository: compozy/compozy

Length of output: 964

---

🏁 Script executed:

```shell
# Check the test function that uses this
rg "newInProcessClientRunStream|inProcessClientRunStream" internal/cli/ -B 5 -A 10
```

Repository: compozy/compozy

Length of output: 3408

---

🏁 Script executed:

```shell
# Look for goroutine leak detection in daemon tests
rg "goroutine leak|goleak" internal/cli/ -B 2 -A 2
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check if there's any context passed to stream operations
rg "RunStream\|stream\." internal/cli/daemon_exec_test_helpers_test.go -B 2 -A 3 | head -60
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Verify the actual blocking scenario - check if items channel could be abandoned
cat internal/cli/daemon_exec_test_helpers_test.go | sed -n '253,320p'
```

Repository: compozy/compozy

Length of output: 1326

---

🏁 Script executed:

```shell
# Check how consumeDaemonRunStream is used - does it exit early?
rg "consumeDaemonRunStream" -B 5 -A 15 internal/cli/reviews_exec_daemon.go
```

Repository: compozy/compozy

Length of output: 2111

---

🏁 Script executed:

```shell
# Check the context usage patterns to see how the stream is typically consumed
rg "streamConsumer|consumeDaemonRunStream|waitForDaemonRunTerminal" -B 2 -A 8 internal/cli/ | head -80
```

Repository: compozy/compozy

Length of output: 6056

---

🏁 Script executed:

```shell
# Verify if there's a way to drain remaining items after early exit
rg "for.*range.*Items\|for.*<-.*Items" internal/cli/ -B 3 -A 5
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Let me examine consumeDaemonRunStream more closely to understand the early exit paths
rg "consumeDaemonRunStream" -A 35 internal/cli/reviews_exec_daemon.go | head -60
```

Repository: compozy/compozy

Length of output: 1692

---

🏁 Script executed:

```shell
# Check if there's any draining of channels happening elsewhere
rg "close(.*items)|drain|for.*range" internal/cli/daemon_exec_test_helpers_test.go
```

Repository: compozy/compozy

Length of output: 79

---

🏁 Script executed:

```shell
# Look for similar stream bridge patterns in the codebase to see how they handle shutdown
rg "go func.*defer close.*for.*select" -A 20 internal/cli/
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check test infrastructure to see if there are any goroutine leak detectors being used
cat internal/cli/reviews_exec_daemon_additional_test.go | grep -A 30 "func TestConsumeStream\|goroutine\|goleak" | head -40
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Let's trace through consumeDaemonRunStream to see what happens on early return
rg "func consumeDaemonRunStream" -A 40 internal/cli/reviews_exec_daemon.go
```

Repository: compozy/compozy

Length of output: 874

---

🏁 Script executed:

```shell
# Check if there's any mechanism to drain the stream after Close()
rg "Close\(\)|defer.*stream" -B 2 -A 2 internal/cli/reviews_exec_daemon.go | head -50
```

Repository: compozy/compozy

Length of output: 104

---

🏁 Script executed:

```shell
# Verify the exact scenario: when handler returns error early, what happens to the goroutine?
cat internal/cli/daemon_exec_test_helpers_test.go | sed -n '223,270p'
```

Repository: compozy/compozy

Length of output: 1160

---

🏁 Script executed:

```shell
# Now let's verify the exact blocking scenario by examining when handle() returns early
rg "if.*handle\(item\).*err.*{" -A 3 internal/cli/reviews_exec_daemon.go
```

Repository: compozy/compozy

Length of output: 174

---

🏁 Script executed:

```shell
# Check if there's any cleanup of leftover items in the stream
rg "drain|discard|flush" internal/cli/daemon_exec_test_helpers_test.go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Let me directly look at what happens when the handler returns early
sed -n '400,445p' internal/cli/reviews_exec_daemon.go
```

Repository: compozy/compozy

Length of output: 1995

---

🏁 Script executed:

```shell
# Trace through a concrete example - waitForDaemonRunTerminal
rg "waitForDaemonRunTerminal" -A 20 internal/cli/reviews_exec_daemon.go | head -40
```

Repository: compozy/compozy

Length of output: 1256

---

🏁 Script executed:

```shell
# Check if newStaticClientRunStream or other test streams have similar issues
rg "func newStaticClientRunStream|newInProcessClientRunStream" -A 30 internal/cli/
```

Repository: compozy/compozy

Length of output: 5375

---

🏁 Script executed:

```shell
# Let's verify the exact scenario: when consumeDaemonRunStream handler returns early
rg "isTerminalDaemonEvent\|EventKindRun" -B 2 -A 2 internal/cli/reviews_exec_daemon.go | head -30
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check if there's a specific test that demonstrates this early exit pattern
rg "waitForDaemonRunTerminal\|EventKindRunCompleted" -B 3 -A 10 internal/cli/reviews_exec_daemon_additional_test.go | head -50
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Verify that the upstream stream continues sending after handler exits
rg "stream.Events\(\)|stream.Errors\(\)" -B 2 -A 5 internal/cli/daemon_exec_test_helpers_test.go
```

Repository: compozy/compozy

Length of output: 244

---



</details>

**Add a shutdown signal to stop the bridge goroutine immediately when `Close()` is called.**

The bridge goroutine blocks on the unbuffered `items` channel when consumers exit early (e.g., after detecting a terminal event). Since `consumeDaemonRunStream` can return before fully draining the stream, and `Close()` only forwards to the upstream stream, the goroutine remains blocked indefinitely, leaking in tests.

Implement explicit shutdown using a `done` channel that the goroutine monitors in its select loop, allowing `Close()` to signal immediate termination.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/daemon_exec_test_helpers_test.go` around lines 253 - 317, The
bridge goroutine in newInProcessClientRunStream can block forever on the
unbuffered items channel when consumers exit early; add a shutdown signal by
adding a done channel to the inProcessClientRunStream struct, have the goroutine
include a case <-done in its select loop to break and stop forwarding, and
update Close() on inProcessClientRunStream to close or signal the done channel
before calling the upstream stream.Close so the bridge exits immediately and
does not leak; make sure items and errors channels are still closed by the
goroutine when shutting down.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c36ff8f6-d8b2-44c7-a1cd-4e6f4795be06 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Why: the in-process wrapper forwards daemon events/errors from the core `apicore.RunStream` into unbuffered client-facing channels from a background goroutine. If the consumer returns early, `consumeDaemonRunStream` defers `Close()`, but that does not unblock a forwarding goroutine already stuck trying to send the next item/error into the wrapper channel. That leaves the goroutine stranded and can leak work in tests.
- Root cause: `newInProcessClientRunStream` has no local shutdown signal for its forwarding goroutine and performs blocking sends into wrapper channels without selecting on stream closure.
- Resolution: added wrapper-local shutdown coordination with a `done` channel and `sync.Once`, made forwarded item/error sends abort when the stream is closed, and added a regression test that proves the wrapper channels close cleanly after early consumer shutdown.
- Verification: `go test ./internal/cli -run 'Test(DaemonStatusRunUsesCommandContextForProbeAndRPCs|DaemonStopRunUsesCommandContextForProbeAndRPCs|NewInProcessClientRunStreamStopsForwarderOnClose)$' -count=1`; `make verify`
