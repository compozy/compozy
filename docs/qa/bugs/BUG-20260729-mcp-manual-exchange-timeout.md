# BUG-20260729-mcp-manual-exchange-timeout: Manual MCP exchange exposed transport timeout details

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Iris
- **Journey Step:** J-mcp-authorize-repair, complete the Paste Tour within one bounded deadline
- **Scenarios:** ET-cli-mcp-auth-manual-exchange
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated CLI exchange-timeout replay

## Summary

`compozy mcp authorize <name> --manual --timeout <duration>` used the documented authorization
timeout while reading stdin and while exchanging the submitted response, but the two phases exposed
different public failures. Input timeout returned the stable MCP authorization error; exchange
timeout exposed the UDS path and a raw HTTP client `context deadline exceeded` diagnostic.

## Reproduction

1. Configure a loopback OAuth MCP provider whose token endpoint accepts the authorization request
   and holds its response longer than the CLI timeout.
2. Run `compozy mcp authorize <name> --manual --timeout 1s -o json` and submit the provider's
   deterministic slow-exchange code through stdin.
3. Compare the final error with a timeout while the command is still waiting for stdin.

**Expected:** Both phases fail with `cli: MCP authorization timed out` while preserving
`context deadline exceeded` for programmatic error inspection.
**Actual:** The exchange phase exposed the raw UDS POST diagnostic and socket path.

## Evidence

- Input, cancellation, bare-code, redirect-URL, and exchange-timeout assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/024-mcp-oauth-endpoints`.

## Fix

- **Root cause:** `exchangeManualMCPAuth` returned the daemon client error directly even when the
  shared authorization context or the returned error was `context.DeadlineExceeded`.
- **Correction:** The CLI boundary now maps only deadline-exceeded exchange failures through the
  same `mcpAuthorizationTimeoutError` used by pending manual input. Non-timeout exchange errors keep
  their original classification.
- **Fix commit:** `351f3535`
- **Regression owner:** `internal/cli/mcp_auth_test.go`,
  `TestMCPAuthorizeManualHonorsTimeout/Should carry the authorization deadline through manual exchange`.

## Verification

- The strengthened canonical regression failed before the production change and passes under
  `go test -race` afterward.
- A rebuilt CLI reached the delayed token endpoint, exited non-zero with the stable authorization
  timeout, preserved the deadline sentinel, emitted no success payload, and did not echo submitted
  OAuth material.

## Compozy Impact Audit

- **Native tools:** no impact; checked `compozy__mcp_status`, `compozy__mcp_auth_status`, toolsets,
  descriptors, schemas, capability gates, and daemon API fallbacks; no IDs or contracts changed.
- **Extensibility and hooks:** no impact; checked MCP sidecars, registries, auth lifecycle, hooks,
  bundles, bridge SDKs, and config lifecycle. Only CLI presentation of an existing deadline changed.
- **Workspace data isolation:** no impact; the changed datum is an ephemeral CLI error. Global and
  workspace target propagation through HTTP, UDS, settings, Vault, and status responses is unchanged.
- **Official Compozy skill:** no content change required; `skills/compozy/references/tools-and-skills.md`
  already states that `--timeout` bounds manual input and exchange, which now matches runtime truth.
