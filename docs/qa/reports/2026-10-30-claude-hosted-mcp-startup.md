# Claude hosted MCP startup

Scope: issue #1050, `ET-compozy-native-tool-invocation`, macOS with the real
`claude-agent-acp` 0.99.1 adapter and native Claude authentication. The lab used a
separate daemon binary, home, socket, workspace, and loopback port. The operator
daemon was not restarted.

## Findings

The existing serializer preserves the hosted executable and omits `type` for the
stdio variant. Both session creation and loading use that serializer. The same
conversion exists in v0.4.5. The adapter copies `server.command` directly; the
reported spontaneous WSL2 `command: "stdio"` corruption was not reproduced here.
No causal attribution to daemon age, registration races, or the adapter is proven.

The observability defect is confirmed: `mcp_activation` only arms the launch nonce.
It does not await a successful hosted bind. The new Claude startup finalizer waits
for a successful initial tool projection and the proxy's authenticated stream request
confirming receipt of the bind response within the existing launch deadline, and
records `mcp_bind`. Failure rejects startup before first-prompt tool guidance.

## Real execution

- Baseline fresh start and resume established hosted binds with 256 tools.
- Updated fresh start and `session/load` established binds and completed real
  `compozy__tool_search` calls. A forwarding wrapper captured only redacted wire
  shape; both routes omitted `type` and retained the daemon executable, `tool mcp`
  arguments, session/nonce argument positions, and `COMPOZY_HOME` environment key.
- A controlled wrapper changed only the hosted command to `stdio`. The real adapter
  completed session creation, but no hosted bind established. `mcp_bind` failed at
  the configured 30-second launch deadline; the prompt command returned failure
  without submitting the model prompt.
- Removing the injected failure allowed the same logical session to bind and call
  the native tool successfully.

The health endpoint still describes a detached logical session as healthy. This
change exposes the failure through startup errors and `mcp_bind`, not a new health
field. It does not prove a fix for the unobserved WSL2 command corruption.

## Focused automated evidence

Race-enabled tests passed in `internal/mcp`, `internal/acp`, and `internal/session`
for hosted bind lifecycle/deadline/cancellation, on-time versus late projection,
renewed reconnect windows, full timer waits, unacknowledged bind rejection, new/load wire shape and diagnostics,
Pi finalizer preservation, accepted-start
failure cleanup (including real helper subprocess exit) and fallback suppression, finalized first-prompt guidance, hosted
injection, and diagnostic
secret omission. Existing subprocess fixtures exercise the actual ACP JSON boundary.

The test-shape checker passed for the modified MCP suite and session helper. It
reports pre-existing direct-case tests in the ACP files; the new/modified cases use
`Should` subtests. Local broad gates and full E2E were not run; CI owns those gates.

Review remediation also verified that a failed Claude bind cancels the hosted
launch during first-prompt binding. The owning manager regression initially exposed
that missing cleanup; the startup finalizer now performs it before returning failure.
The revised binary repeated the injected failure, recovery, and `session/load`
journey; recovery and resume both completed real native tool calls. The final
receipt-confirmation version also completed fresh and loaded Claude sessions with
real native tool calls, using the existing projection stream as acknowledgment.

Lab teardown completed with `clean: true`. The strict evidence audit's local-gate
requirement remains intentionally unsatisfied under the session instruction that CI
owns broad gates; focused checks and real execution are recorded above.
