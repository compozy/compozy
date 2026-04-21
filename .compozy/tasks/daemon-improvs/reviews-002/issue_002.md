---
status: resolved
file: internal/daemon/boot_integration_test.go
line: 353
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58ixrp,comment:PRRC_kwDORy7nkc654MD-
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Use the repo’s `Should...` subtest naming convention.**

The new subtests use names like `"force=false"` and `"foreground mirrors to stderr"`, so their failure output won’t match the suite’s standard pattern.

As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/boot_integration_test.go` around lines 283 - 353, Rename the
subtest names to follow the repository's "Should..." convention: in
TestManagedDaemonStopEndpointShutsDownAndRemovesSocket change
t.Run("force="+strconv.FormatBool(force), ...) to something like t.Run("Should
stop and remove socket when force="+strconv.FormatBool(force), ...) (referencing
the force loop and StopDaemon/QueryStatus/paths.SocketPath logic), and in
TestManagedDaemonRunModesControlLogging change t.Run("foreground mirrors to
stderr", ...) to t.Run("Should mirror logs to stderr in foreground mode", ...)
and t.Run("detached writes only file", ...) to t.Run("Should write logs only to
file in detached mode", ...) (these refer to startManagedDaemonHelperProcess,
RunModeForeground/RunModeDetached, waitForLogContains and
waitForStderrContains). Ensure no other behavior changes beyond renaming the
t.Run strings.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e40d238e-ad07-4f04-8bea-f476de16b781 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the new subtests in `internal/daemon/boot_integration_test.go` use ad-hoc names instead of the repo’s common `Should ...` naming style used across newer tests, which makes failures less consistent to scan.
- Plan: rename only the affected `t.Run(...)` strings to `Should ...` phrases without changing the test logic.
- Resolution: renamed the stop-endpoint and run-mode subtests in `internal/daemon/boot_integration_test.go` to `Should ...` descriptions and left their behavior unchanged.
- Regression coverage: `TestManagedDaemonStopEndpointShutsDownAndRemovesSocket` and `TestManagedDaemonRunModesControlLogging` still exercise the same detached and foreground helper-process flows.
- Verification: `go test ./internal/daemon -run 'Test(ManagedDaemonStopEndpointShutsDownAndRemovesSocket|ManagedDaemonRunModesControlLogging)$' -count=1` passed. `make verify` then passed with `2544` tests and `2` skipped helper-process tests.
