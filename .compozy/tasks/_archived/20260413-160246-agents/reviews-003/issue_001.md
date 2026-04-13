---
status: resolved
file: internal/core/agents/mcpserver/server_test.go
line: 190
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QZsS,comment:PRRC_kwDORy7nkc620G-v
---

# Issue 001: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify whether ServeStdio writes to stdout paths that can block with an undrained pipe.
rg -n --type=go -C3 '\bfunc\s+ServeStdio\s*\('
rg -n --type=go -C3 'os\.Stdout|fmt\.Fprint|Write\(|json\.NewEncoder'
```

Repository: compozy/compozy

Length of output: 50371

---

🏁 Script executed:

```shell
# Get the actual test code at lines 158-190
sed -n '140,195p' internal/core/agents/mcpserver/server_test.go

# Also check the RunStdio implementation
rg -n 'func.*RunStdio' internal/core/agents/mcpserver/ -A 30
```

Repository: compozy/compozy

Length of output: 3605

---

🏁 Script executed:

```shell
# Check mcp imports in server.go
head -30 internal/core/agents/mcpserver/server.go

# Search for any immediate stdout writes in the server setup
rg -n 'os\.Stdout|fmt\.Fprint' internal/core/agents/mcpserver/ --type go

# Check if mcp.StdioTransport is from an external package
rg -n 'import.*mcp' internal/core/agents/mcpserver/
```

Repository: compozy/compozy

Length of output: 1023

---

🏁 Script executed:

```shell
# Check what happens in mcp.StdioTransport - search for any initialization writes
# First check if there are other similar test patterns in the codebase
rg -n 'os\.Pipe\(\)' internal/core/agents/mcpserver/server_test.go -A 10 -B 2

# Check if there's any initialization output from mcp in examples
rg -n 'mcp.StdioTransport' --type go -B 5 -A 5
```

Repository: compozy/compozy

Length of output: 1395

---

🏁 Script executed:

```shell
# Check if there's any init output from mcp.NewServer or related functions
# Let's look at similar test patterns in the codebase for draining
rg -n 'io\.Copy.*io\.Discard' --type go

# Check what other tests do with pipe draining
rg -n 'stdoutRead|stderrRead' internal/core/ --type go -A 3 | head -100
```

Repository: compozy/compozy

Length of output: 4011

---

🏁 Script executed:

```shell
# Check the readPipe function implementation
rg -n 'func readPipe' internal/core/ --type go -A 10
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Search for readPipe definition
rg -n 'readPipe' internal/core/ --type go -B 2 -A 8 | head -60
```

Repository: compozy/compozy

Length of output: 4098

---

🏁 Script executed:

```shell
# Get the full readPipe function
sed -n '1350,1370p' internal/core/run/executor/execution_acp_integration_test.go
```

Repository: compozy/compozy

Length of output: 612

---

🏁 Script executed:

```shell
# Check if mcp library is vendored or external, and look for any initialization writes
# Let's trace through what happens when server.Run is called
rg -n 'go-sdk/mcp' --type go

# Let's also check if there are any writes in the test setup or mcp usage
# Look at how mcp.StdioTransport initializes
find . -path ./vendor -prune -o -name '*.go' -type f -exec grep -l 'StdioTransport' {} \;
```

Repository: compozy/compozy

Length of output: 372

---

🏁 Script executed:

```shell
# Look for any writes in mcp server initialization - check if json encoder is used
rg -n 'json\.NewEncoder|json\.Marshal|fmt\.Fprint' internal/core/agents/mcpserver/ --type go

# Check the test more carefully - is stdinWrite closed to signal EOF?
sed -n '150,195p' internal/core/agents/mcpserver/server_test.go
```

Repository: compozy/compozy

Length of output: 1356

---

🏁 Script executed:

```shell
# Check if stdinWrite is ever closed or written to before ServeStdio is called
# Also check if there's a similar pattern in other tests with proper draining
rg -n 'stdinWrite.Close\|stdinWrite.Write' internal/core/agents/mcpserver/server_test.go

# Check what happens with mcp protocol - does it write initial messages?
# Look at other MCP tests to see how they handle stdio pipes
rg -n 'mcp\.StdioTransport\|NewServer' internal/core/agents/mcpserver/ -B 3 -A 5
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check if stdinWrite is closed before the test finishes
# and look at the exact sequence and timing
sed -n '150,192p' internal/core/agents/mcpserver/server_test.go

# Verify: what does the mcp protocol library do on initialization?
# Look for any patterns in how this test expects the goroutine to behave
rg -n 'stdinWrite.Close\(\)' internal/core/agents/mcpserver/server_test.go
```

Repository: compozy/compozy

Length of output: 1145

---

🏁 Script executed:

```shell
# Check if there are any writes that happen during mcp.NewServer or server.Run
# Let's look at what the MCP server initialization does
# First, check if there's documentation or other uses that show what happens on startup

# Look at test patterns - does result_test.go properly drain pipes?
sed -n '229,255p' internal/core/run/executor/result_test.go
```

Repository: compozy/compozy

Length of output: 709

---

🏁 Script executed:

```shell
# Let's trace the execution path: what does ServeStdio -> RunStdio -> server.Run do?
# Check if there's initialization output from mcp before it reads stdin

# Look at how execution_acp_integration_test properly handles this
sed -n '1324,1390p' internal/core/run/executor/execution_acp_integration_test.go
```

Repository: compozy/compozy

Length of output: 1710

---

🏁 Script executed:

```shell
# Run the specific test to see if it passes
cd internal/core/agents/mcpserver && go test -timeout 10s -run TestServeStdioReturnsWhenContextIsCanceled -v 2>&1 | head -50
```

Repository: compozy/compozy

Length of output: 1767

---

🏁 Script executed:

```shell
# Check if mcp.StdioTransport writes any initialization message
# Look at what happens when stdinWrite is not closed (EOF not sent)
# Let's check if there are any comments or documentation about this

# Also check: does the test pass with -race flag?
cd internal/core/agents/mcpserver && go test -race -timeout 10s -run TestServeStdioReturnsWhenContextIsCanceled 2>&1 | tail -20
```

Repository: compozy/compozy

Length of output: 118

---

🏁 Script executed:

```shell
# Let's verify the concern by checking if stdinWrite needs to be closed to signal EOF
# Check if context cancellation alone is enough or if the server tries to read stdin
sed -n '64,77p' internal/core/agents/mcpserver/server.go

# Also check if there are any writes in the mcp server setup
# by looking at what happens during initialization
rg -n 'impl\(\)|AddTool' internal/core/agents/mcpserver/server.go -B 2 -A 2
```

Repository: compozy/compozy

Length of output: 1126

---



</details>

**Drain the stdout pipe to prevent potential test flakiness.**

The test redirects stdout to a pipe but never reads from `stdoutRead`. If `mcp.StdioTransport` or the MCP server writes any output before context cancellation, the goroutine could block on the write and cause the test to hang. Other similar tests in the codebase (e.g., `execution_acp_integration_test.go`, `result_test.go`) drain pipes using `io.ReadAll` in a background goroutine. Apply the same pattern here for consistency and robustness.

<details>
<summary>Suggested minimal fix (drain stdout pipe)</summary>

```diff
 import (
 	"context"
 	"encoding/json"
 	"errors"
+	"io"
 	"os"
 	"strings"
 	"sync"
@@
 	stdoutRead, stdoutWrite, err := os.Pipe()
 	if err != nil {
 		t.Fatalf("create stdout pipe: %v", err)
 	}
+	drainDone := make(chan struct{})
+	go func() {
+		_, _ = io.Copy(io.Discard, stdoutRead)
+		close(drainDone)
+	}()
@@
 	defer stdoutRead.Close()
 	defer stdoutWrite.Close()
+	defer func() { <-drainDone }()
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/mcpserver/server_test.go` around lines 158 - 190, The
test currently redirects stdout to a pipe but never consumes stdoutRead, which
can cause ServeStdio to block on writes; spawn a background goroutine that calls
io.ReadAll(stdoutRead) (or otherwise continuously drains stdoutRead) immediately
after creating the pipe and before calling ServeStdio so the pipe buffer can't
fill and block the server; reference stdoutRead/stdoutWrite and ServeStdio(ctx,
HostContext{}) when adding the drain goroutine and ensure it runs until
stdoutWrite is closed.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ec4509db-bb77-400a-bf07-325abcc03a3b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `INVALID`
- Notes:
  - Verified against the current SDK implementation in `github.com/modelcontextprotocol/go-sdk@v1.5.0/mcp/transport.go` and `mcp/server.go`.
  - `mcp.StdioTransport.Connect` only binds `os.Stdin` and `os.Stdout`; it does not emit any startup bytes.
  - `Server.Run` waits on context cancellation or session termination and does not send protocol output before a client request initializes the session.
  - The current test cancels immediately, and a focused run of `go test ./internal/core/agents/mcpserver -run TestServeStdioReturnsWhenContextIsCanceled -count=1 -v` passes without any blocked write path.
  - Adding a drain goroutine here would be speculative hardening, not a root-cause fix for a real issue in the current code path.
