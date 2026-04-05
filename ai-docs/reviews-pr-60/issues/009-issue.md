# Issue 9 - Review Thread Comment

**File:** `internal/core/run/exec_flow.go:437`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** INVALID

**Rationale:** The streaming goroutine is already owned by the session lifetime: session completion closes `Updates()`, and `execution.close()` closes the ACP client on exit. Adding `attemptCtx.Done()` to the receive loop would let cancellation race ahead of terminal session updates and could drop completion or failure state that the current flow still drains through the session.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify goroutine lifecycle handling around streamExecSession.
rg -n -C4 'streamExecSession\(' internal/core/run/exec_flow.go
rg -n -C6 'go func\(\)' internal/core/run/exec_flow.go
rg -n -C6 'case <-attemptCtx.Done\(\)' internal/core/run/exec_flow.go
```

Repository: compozy/compozy

Length of output: 1562

---

🏁 Script executed:

```shell
#!/bin/bash
# Check for WaitGroup or other synchronization in exec_flow.go
rg -n 'WaitGroup|wg\.' internal/core/run/exec_flow.go

# Check all calls to streamExecSession
rg -n 'streamExecSession' internal/core/run/exec_flow.go

# Check the full function signature and context usage
rg -n -A30 'func execExecution\(' internal/core/run/exec_flow.go | head -50
```

Repository: compozy/compozy

Length of output: 206

---

🏁 Script executed:

```shell
#!/bin/bash
# Get full function containing the reviewed code
rg -n -B30 'streamErrCh := streamExecSession' internal/core/run/exec_flow.go | head -60

# Check sessionExecution type and Done() method
rg -n 'type sessionExecution\|func.*Done\(' internal/core/run/exec_flow.go | head -20

# Check Updates() method
rg -n 'func.*Updates\(' internal/core/run/exec_flow.go
```

Repository: compozy/compozy

Length of output: 825

---

🏁 Script executed:

```shell
#!/bin/bash
# Find sessionExecution type definition
rg -n 'type sessionExecution struct' internal/core/run/exec_flow.go

# Find close() method
rg -n 'func.*\*sessionExecution\).*close' internal/core/run/exec_flow.go

# Find Done() method
rg -n 'func.*\).*Done\(\)' internal/core/run/exec_flow.go

# Check if sessionExecution or related types are defined elsewhere
fd -t f '\.go$' internal/core/run | head -20
```

Repository: compozy/compozy

Length of output: 771

---

🏁 Script executed:

```shell
#!/bin/bash
# Find sessionExecution type definition
rg -n 'type sessionExecution' internal/core/run/execution.go

# Find close method on sessionExecution
rg -n -A5 'func.*sessionExecution.*close' internal/core/run/execution.go

# Find Done method - check session object
rg -n 'session.*Done\|Done.*chan' internal/core/run/execution.go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
#!/bin/bash
# Search more broadly for sessionExecution
rg -n 'sessionExecution' internal/core/run/execution.go | head -20

# Check types.go for the definition
rg -n 'sessionExecution\|type.*session' internal/core/run/types.go | head -20

# Search for the close method
rg -n '\.close\(\)' internal/core/run/exec_flow.go
```

Repository: compozy/compozy

Length of output: 207

---

🏁 Script executed:

```shell
#!/bin/bash
# Search for type sessionExecution definition in execution.go
cat internal/core/run/execution.go | grep -n 'type sessionExecution' -A10

# Or use ast-grep to find struct definition
ast-grep --pattern 'type sessionExecution struct { $$$ }'

# Search for close method definition
rg -n -B2 -A10 'func \(.*\*sessionExecution\) close' internal/core/run/
```

Repository: compozy/compozy

Length of output: 1273

---

🏁 Script executed:

```shell
#!/bin/bash
# Continue reading the close method
rg -n -A15 'func \(s \*sessionExecution\) close' internal/core/run/command_io.go

# Check if session has a Close or Done method
rg -n 'session.*Close\|session.*Done' internal/core/run/command_io.go

# Search for agent.Session interface definition
rg -n 'type Session interface' --type-list all | head -5
```

Repository: compozy/compozy

Length of output: 851

---

🏁 Script executed:

```shell
#!/bin/bash
# Find agent.Session interface definition (likely in a different package)
find . -name '*.go' -type f | xargs rg -l 'type Session interface' | head -5

# Check what the session field is and how Done() works
rg -n 'session\.Done\(\)' internal/core/run/exec_flow.go

# Look for the Updates method signature
rg -n 'Updates\(\)' internal/core/run/ | head -10

# Check imports in command_io.go or exec_flow.go to find agent package
rg -n '^import\|".*agent' internal/core/run/command_io.go internal/core/run/exec_flow.go | head -15
```

Repository: compozy/compozy

Length of output: 746

---

🏁 Script executed:

```shell
#!/bin/bash
# Check the agent.Session interface definition
rg -n -A20 'type Session interface' ./internal/core/agent/session.go

# Verify the Done() method signature
rg -n 'Done\(\)' ./internal/core/agent/session.go

# Check if Updates() and Done() channels are somehow connected
rg -n -B5 -A10 'func.*Updates\|func.*Done' ./internal/core/agent/session.go | head -40
```

Repository: compozy/compozy

Length of output: 872

---

🏁 Script executed:

```shell
#!/bin/bash
# Check the sessionImpl.Done() implementation and how it relates to context
rg -n -B5 -A15 'func \(s \*sessionImpl\) Done' ./internal/core/agent/session.go

# Check how sessionImpl is created and if context is passed
rg -n 'type sessionImpl struct' -A10 ./internal/core/agent/session.go
```

Repository: compozy/compozy

Length of output: 797

---

🏁 Script executed:

```shell
#!/bin/bash
# Check if Updates channel is closed when session is done
rg -n 'close.*updates\|updates.*close' ./internal/core/agent/session.go

# Verify how the Updates channel is managed when Done() fires
rg -n -B10 -A10 'func \(s \*sessionImpl\) publish\|s.finished' ./internal/core/agent/session.go | head -40
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
#!/bin/bash
# Search for where done channel is closed/signaled
rg -n 'done.*<-\|close(.*done\|s.done' ./internal/core/agent/session.go

# Search for where updates channel is closed
rg -n 'close(.*updates\|updates.*close' ./internal/core/agent/session.go

# Look at the full sessionImpl implementation
head -200 ./internal/core/agent/session.go | tail -100
```

Repository: compozy/compozy

Length of output: 2448

---

🏁 Script executed:

```shell
#!/bin/bash
# Verify the exact flow when attemptCtx is canceled
rg -n -B5 -A20 'case <-attemptCtx.Done' internal/core/run/exec_flow.go
```

Repository: compozy/compozy

Length of output: 915

---

🏁 Script executed:

```shell
#!/bin/bash
# Check what completeFinishedExecAttempt and failExecAttempt do with streamErrCh
rg -n -A10 'func completeFinishedExecAttempt\|func failExecAttempt' internal/core/run/exec_flow.go

# Check if there's any mechanism to drain or close the channel
rg -n 'streamErrCh' internal/core/run/exec_flow.go
```

Repository: compozy/compozy

Length of output: 389

---



</details>

**Session stream goroutine lacks explicit cancellation ownership.**

`streamExecSession` launches a long-running goroutine that ranges over updates without responding to context cancellation. When `attemptCtx` is canceled or times out, the function returns immediately, but the goroutine continues running until `execution.session.Updates()` closes. This violates the coding guideline requiring explicit context-based shutdown for all goroutines.

<details>
<summary>Proposed fix</summary>

```diff
-	streamErrCh := streamExecSession(execution, state)
+	streamErrCh := streamExecSession(attemptCtx, execution, state)

-func streamExecSession(execution *sessionExecution, state *execRunState) <-chan error {
+func streamExecSession(ctx context.Context, execution *sessionExecution, state *execRunState) <-chan error {
 	streamErrCh := make(chan error, 1)
 	go func() {
-		for update := range execution.session.Updates() {
-			if err := execution.handler.HandleUpdate(update); err != nil {
-				streamErrCh <- err
-				return
-			}
-			if state != nil {
-				if err := state.emitSessionUpdate(update); err != nil {
-					streamErrCh <- err
-					return
-				}
-			}
-		}
-		streamErrCh <- nil
+		for {
+			select {
+			case <-ctx.Done():
+				streamErrCh <- context.Cause(ctx)
+				return
+			case update, ok := <-execution.session.Updates():
+				if !ok {
+					streamErrCh <- nil
+					return
+				}
+				if err := execution.handler.HandleUpdate(update); err != nil {
+					streamErrCh <- err
+					return
+				}
+				if state != nil {
+					if err := state.emitSessionUpdate(update); err != nil {
+						streamErrCh <- err
+						return
+					}
+				}
+			}
+		}
 	}()
 	return streamErrCh
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec_flow.go` around lines 405 - 437, The goroutine in
streamExecSession can leak because it never observes context cancellation;
change streamExecSession to accept the attempt context (e.g.,
streamExecSession(ctx context.Context, execution *sessionExecution, state
*execRunState)), and inside the goroutine replace the range over
execution.session.Updates() with an explicit loop that selects on ctx.Done() and
a receive from execution.session.Updates(); when ctx is done return (and send
ctx.Err() on streamErrCh), otherwise handle the update as before (calling
execution.handler.HandleUpdate and state.emitSessionUpdate). Update the call
site to pass attemptCtx into streamExecSession so the goroutine stops promptly
when attemptCtx is canceled.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:443cb2f9-9289-4e80-a9c6-9308c8d22d24 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHn`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHn
```

---
*Generated from PR review - CodeRabbit AI*
