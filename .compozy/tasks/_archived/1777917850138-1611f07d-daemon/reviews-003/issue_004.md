---
status: resolved
file: internal/cli/operator_commands_integration_test.go
line: 505
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_RAK,comment:PRRC_kwDORy7nkc65JPyV
---

# Issue 004: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Avoid selecting a TCP port by closing it before the daemon starts.**

This helper only proves the port was free momentarily. After `listener.Close()`, another process can claim it before the daemon binds, so these integration tests will fail intermittently on busy CI hosts. Prefer asserting the daemon’s reported port after startup, or plumbing an already-open listener/FD into the child process instead of racing on a released port.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/operator_commands_integration_test.go` around lines 485 - 505,
The helper configureCLITestDaemonHTTPPort currently closes the ephemeral
listener before the daemon starts, which races with other processes; instead
keep the listener open and hand its file descriptor or address into the daemon
startup (or return the open listener) so the child process binds the
already-open socket, or change the test to start the daemon first and then
read/assert the daemon’s reported port (using daemonHTTPPortEnv or the daemon’s
status output) rather than selecting and releasing the port beforehand; update
call sites of configureCLITestDaemonHTTPPort to accept an open listener/port
reported after startup and ensure listener.Close() happens only after the daemon
has bound or the test finishes.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:13e5f7e2-231d-4efa-b0c2-160e6a6e0bff -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `configureCLITestDaemonHTTPPort` currently asks the OS for a free port by listening on `127.0.0.1:0`, captures that port number, closes the listener, and only then starts the daemon. That gap reintroduces a race because another process can bind the released port before the daemon does.
- Fix approach: remove the pre-bind reservation pattern entirely. Let the test request an OS-assigned daemon HTTP port explicitly, then assert the daemon-reported `http_port` after startup and on subsequent status reads. This requires a small daemon startup change so an explicit `COMPOZY_DAEMON_HTTP_PORT=0` means "bind an ephemeral port" without changing the default behavior when the env var is unset.
- Resolution: `internal/cli/daemon.go` now maps an explicit `COMPOZY_DAEMON_HTTP_PORT=0` to `daemon.EphemeralHTTPPort`; `internal/daemon/info.go` and `internal/daemon/boot.go` preserve that explicit ephemeral-port request while still defaulting the unset path to `2323`; and `internal/cli/operator_commands_integration_test.go` now requests the ephemeral port via env and asserts the daemon-reported `http_port` instead of racing on a preselected released port.
- Regression coverage: `TestNormalizeStartOptionsUsesEphemeralHTTPPortWhenRequested` in `internal/daemon/boot_test.go` locks the daemon normalization path, and the updated CLI integration flow in `TestDaemonStatusAndStopCommandsOperateAgainstRealDaemon` plus the other `configureCLITestDaemonHTTPPort` callers exercise the explicit-ephemeral-port path end to end.
- Verification: `go test ./internal/daemon -run 'Test(StartDefaultsHTTPPortWhenUnset|NormalizeStartOptionsUsesEphemeralHTTPPortWhenRequested)$' -count=1` passed; `go test ./internal/cli -run 'Test(DaemonStatusAndStopCommandsOperateAgainstRealDaemon|WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon|SyncAndArchiveCommandsUseDaemonStateFromWorkspaceSubdirectory|ArchiveCommandArchivesSyncedWorkflowIntoNewPathFormat)$' -count=1` passed; and `make verify` passed after the final code and test updates, with `2403` tests run, `1` skipped daemon helper-process case, and a successful `go build ./cmd/compozy`.
