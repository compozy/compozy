# BUG-20260826-terminal-journal-workspace-id: Terminal journal rejects the workspace shown by the product

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Rafa
- **Journey Step:** J-audit-terminal-work, open the journal for the active workspace
- **Scenarios:** ET-terminal-journal-recording; ET-terminal-cli-public-contract
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-integrated-terminal.md

## Summary

An operator who opens the terminal journal for the workspace shown by Compozy receives an internal
error instead of history or an empty result. The same workspace works for the terminal catalog, so the
public surfaces disagree about which workspace the operator is using.

## Reproduction

- **Charter:** CH-terminal-journal-fail-closed · **Tour:** Network Tour
- **Environment:** isolated macOS runtime harness / desktop / wifi-fast / en-US

1. Create an agent-backed session in a registered workspace.
2. Produce agent-reported terminal output without creating a supervised terminal.
3. Read the terminal catalog through UDS and observe the expected empty list.
4. Read the journal through UDS using the same public workspace id.

**Expected:** The journal returns an empty page because reported output is observational only.
**Actual:** The route returns HTTP 500 because it compares the public `ws_*` registration id with the workspace directory's internal ULID.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime.log`
- Focused independent reproduction: `go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2EAgentReportedTerminalStaysObservational$' -count=1 -v` failed with the same mismatch under a fresh harness workspace.

## Fix

- **Root cause:** The terminal runtime treated the workspace directory identity as the public workspace id, while the workspace database pool assumed its lookup key and on-disk identity were the same value. The workspace model deliberately keeps those identities separate.
- **Fix commit:** `b745ebcbcfe6`
- **Regression test:** `internal/daemon/daemon_mock_agents_integration_test.go` — `TestDaemonE2EAgentReportedTerminalStaysObservational` failed twice before the fix.

## Verification

- **Retested:** 2026-08-26 with the focused daemon integration test and the complete runtime E2E lane.
- **Result:** pass — daemon 175/175, HTTP 21/21, UDS 46/46, harness 8/8, and CLI 4/4.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log`
