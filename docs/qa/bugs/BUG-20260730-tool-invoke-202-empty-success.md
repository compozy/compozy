# BUG-20260730-tool-invoke-202-empty-success: Tool invoke reports approval-required as empty success

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada, Bruno
- **Journey Step:** J-cross-workspace-access, invoke a native tool that requires approval
- **Scenarios:** ET-workspace-access-mode-matrix; ET-workspace-access-prompt-outcomes
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-29-site-improvs-deep-review.md

## Summary

`compozy tool invoke` treated the daemon's HTTP 202 approval-required response as a successful tool result. The generic JSON client accepted every 2xx status and decoded the error envelope into an empty success record, hiding the pending approval and its reason codes.

## Reproduction

- **Charter:** CH-cross-workspace-mode-seams · **Tour:** Feature Tour
- **Environment:** isolated `site-improvs-deep-review` lab with two registered workspaces and a real `deny-all` agent session

1. Invoke `compozy__workspace_info` through `compozy tool invoke` without an approval token.
2. Compare the CLI result with the raw UDS response.

**Expected:** The CLI returns `tool_approval_required` with the tool ID and daemon reason codes.
**Actual:** The CLI decoded the HTTP 202 envelope as an empty successful invocation.

## Evidence

- Raw UDS response contained `tool_approval_required`, `approval_token_missing`, and `approval_required`.
- The corrected CLI returned the same structured error in the isolated lab.

## Fix

- **Root cause:** `UnixSocketClient.InvokeTool` delegated to the generic `doJSON` helper, whose success range includes HTTP 202.
- **Fix:** `InvokeTool` now handles HTTP 202 as a structured API error before decoding successful tool results.
- **Fix commit:** Working tree; this review task did not authorize a commit.
- **Regression gate:** `internal/cli/client_test.go` owns the transport invariant that approval-required responses never decode as zero-valued success.

## Verification

- `go test -race ./internal/cli -run TestUnixSocketClientToolMethods -count=1`
- Real CLI retest returned one structured `tool_approval_required` error with the original tool ID and reason codes.
