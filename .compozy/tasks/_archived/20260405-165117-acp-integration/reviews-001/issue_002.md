---
status: resolved
file: internal/core/agent/process_unix.go
line: 10
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHBm,comment:PRRC_kwDORy7nkc60y0fc
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`According to the Go standard library docs for `os.Process.Kill`, `os.ErrProcessDone`, and `syscall.Kill`, should callers use `errors.Is(err, os.ErrProcessDone)` / `errors.Is(err, syscall.ESRCH)` instead of direct equality when handling benign process-termination races?`

💡 Result:

Yes, callers should use errors.Is(err, os.ErrProcessDone) when handling errors from os.Process.Kill (and os.Process.Signal) to detect benign process-termination races where the process has already finished. Direct equality err == os.ErrProcessDone is insufficient because the underlying syscall.Kill may return syscall.ESRCH wrapped in a syscall error, and os.Process.signal explicitly maps syscall.ESRCH to os.ErrProcessDone (or an equivalent sentinel), but errors.Is handles both the sentinel and any wrapped errors properly. For direct syscall.Kill(pid, sig), use errors.Is(err, syscall.ESRCH) since it returns a syscall.Errno, which implements errors.Is to unwrap targets correctly. Direct equality err == syscall.ESRCH works for simple cases but errors.Is is more robust for wrapped errors. This practice is evidenced by official Go source code using errors.Is(err, os.ErrProcessDone) in os/exec (e.g., lines checking interruptErr and killErr) and tools like golang.org/x/tools, confirming it's the recommended approach for handling these races portably across Unix-like systems.

Citations:

- 1: https://pkg.go.dev/os
- 2: https://go.pact.im/doc/use/os..ErrProcessDone.html
- 3: https://pkg.go.dev/os/
- 4: https://go.dev/issue/39444
- 5: https://pkg.go.dev/os@go1.25.5
- 6: https://go.dev/src/os/exec_unix.go
- 7: https://pkg.go.dev/syscall

---



</details>

**Use `errors.Is()` for process-termination checks.**

Direct equality comparisons `killErr == syscall.ESRCH` and `err != os.ErrProcessDone` violate the guideline to use `errors.Is()` for error matching. These comparisons may miss wrapped errors during process-termination races. Additionally, wrap the fallback error with context per "Prefer explicit error returns with wrapped context using `fmt.Errorf("context: %w", err)`".

<details>
<summary>Suggested fix</summary>

```diff
 import (
+	"errors"
 	"fmt"
 	"os"
 	"os/exec"
 	"syscall"
 )
@@
 	pgid, err := syscall.Getpgid(cmd.Process.Pid)
 	if err == nil && pgid > 0 {
-		if killErr := syscall.Kill(-pgid, syscall.SIGKILL); killErr == nil || killErr == syscall.ESRCH {
+		if killErr := syscall.Kill(-pgid, syscall.SIGKILL); killErr == nil || errors.Is(killErr, syscall.ESRCH) {
 			return nil
 		}
 	}
-	if err := cmd.Process.Kill(); err != nil && err != os.ErrProcessDone {
-		return err
+	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
+		return fmt.Errorf("kill ACP process %d: %w", cmd.Process.Pid, err)
 	}
 	return nil
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/process_unix.go` around lines 5 - 10, Replace direct
error comparisons (e.g., killErr == syscall.ESRCH and err != os.ErrProcessDone)
with errors.Is checks (errors.Is(killErr, syscall.ESRCH) and !errors.Is(err,
os.ErrProcessDone)); import the "errors" package. Also wrap the fallback error
when returning/propagating (use fmt.Errorf("context: %w", err)) so the original
error is preserved. Look for the process-termination logic that sets killErr and
err (e.g., in the terminate/kill routine in this file) and update those
comparisons and return statements accordingly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:9505bd09-f35d-4087-8d05-bc08c4654a2d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: `forceTerminateProcess` compares termination races with direct equality (`== syscall.ESRCH`, `!= os.ErrProcessDone`), which is brittle when the error is wrapped. This file should follow the repository's normal `errors.Is` matching and wrap the fallback kill failure with context.
- Fix plan: Switch the benign process-termination checks to `errors.Is` and wrap the fallback `Process.Kill` error with `fmt.Errorf`.
- Resolution: Updated the Unix helper to use `errors.Is` for benign termination races and to wrap fallback kill failures with process context. `make verify` passed.
