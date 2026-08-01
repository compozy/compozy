# BUG-20260801-mcp-manual-input-timeout: Manual MCP login ignored its input deadline

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Iris
- **Journey Step:** J-mcp-authorize-repair, wait for the manual redirect under one bounded deadline
- **Scenarios:** ET-cli-mcp-auth-manual-exchange
- **Found:** 2026-08-01 · **Report:** docs/qa/reports/2026-08-01-loops-paper-task01-mcp-manual-timeout.md
- **Origin:** Full gate timeout followed by a fresh isolated public-provider replay

## Summary

`compozy mcp auth login <name> --manual --timeout <duration>` could remain blocked on inherited
stdin after its documented authorization deadline. The deadline governed the OAuth begin and
exchange requests, but a pending manual read could keep the command alive indefinitely.

## Reproduction

- **Charter:** CH-remote-operator-manual-auth · **Tour:** Paste Tour
- **Environment:** isolated macOS lab / CLI + HTTP + UDS / deterministic OAuth authority over public
  HTTPS / en-US

1. Configure a pre-registered OAuth MCP server and begin manual login with `--timeout 1s`.
2. Redirect stdin from a FIFO whose writer keeps the pipe open without sending bytes.
3. Observe the command after the one-second authorization deadline, then read auth status through
   both the CLI and HTTP API.

**Expected:** The command exits non-zero at the shared deadline with
`cli: MCP authorization timed out: context deadline exceeded`; stdin remains open and blocking, no
token exchange occurs, and status remains `needs_login` with `token_present:false`.

**Actual before the fix:** The full gate left the read blocked past its 30-minute package timeout.
The first partial correction avoided the deadlock in an `os.Pipe` regression but inherited FIFO
stdin then failed immediately with `file type does not support deadline` instead of honoring the
requested timeout.

## Evidence

- Initial blocked goroutine: `.cache/gate/logs/full-1785566653.log`.
- Public-provider replay, exit duration, status post-condition, and redaction scan:
  `/Users/pedronauck/dev/qa-labs/compozy-loops-paper-task01-mcp-manual-timeout-20260801-075922-632496-lab/qa-artifacts/qa/notes/manual-input-timeout-public.json`.

## Fix

- **Root cause:** terminal detection called `os.File.Fd`, which converts Go-managed pollable files
  back to blocking mode. Inherited process stdin is also represented as a blocking `os.File`, so
  `SetReadDeadline` returns `os.ErrNoDeadline`; closing a borrowed descriptor from another goroutine
  is not a reliable cancellation mechanism.
- **Correction:** terminal detection now inspects the descriptor through `SyscallConn`. When an
  inherited non-terminal file has no Go deadline support, the reader duplicates its descriptor,
  temporarily makes the shared file description nonblocking, registers the duplicate with Go's
  poller, interrupts it with a read deadline, closes the owned duplicate, and restores the borrowed
  descriptor's original flags. Expected poller timeout errors collapse into the canonical public
  authorization-timeout error.
- **Fix commit:** 38b2d40
- **Regression owner:** `internal/cli/mcp_auth_test.go`,
  `TestMCPAuthLoginManualHonorsTimeout/Should interrupt pending manual input at the authorization deadline`.

## Verification

- The strengthened canonical regression uses inherited blocking pipe semantics, asserts the exact
  public timeout, and proves the borrowed file remains open and blocking after cancellation.
- `go test -race ./internal/cli -run '^TestMCPAuthLoginManualHonorsTimeout$' -count=1` and the complete
  `internal/cli` race suite pass; the Windows CLI test binary cross-compiles successfully.
- The rebuilt CLI exited in 0.966 seconds with only the canonical timeout. Independent CLI and HTTP
  reads remained `needs_login` with no token, the provider received no token request, and the
  artifact/runtime secret scan found no access token, refresh token, verifier, client secret, or
  authorization code.

## Compozy Impact Audit

- **Native tools:** no impact; checked `compozy__mcp_status`, `compozy__mcp_auth_status`, MCP toolsets,
  descriptors, schemas, capability gates, and CLI/API fallbacks. No native ID or contract changed.
- **Extensibility and hooks:** no impact; checked MCP sidecars, registries, auth lifecycle, hooks,
  bundles, bridge SDKs, and config lifecycle. The correction is confined to local CLI input
  cancellation for the existing manual-auth flow.
- **Workspace data isolation:** no impact; the changed datum is ephemeral CLI input. The replay used
  a global target and proved the timeout created no credential; global/workspace target propagation
  through HTTP, UDS, settings, Vault, and status responses is unchanged.
- **Official Compozy skill:** no content change required; the bundled skill already states that
  `--timeout` bounds manual input and exchange, and the runtime now satisfies that contract.
