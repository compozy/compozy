# BUG-20260715-mcp-manual-tty-echo: Manual MCP authorization echoes secret callback input in a TTY

- **Status:** verified
- **Impact (user-side):** Leaks secret-class exchange input
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Iris
- **Journey Step:** J-mcp-authorize-repair, remote manual exchange
- **Scenarios:** ET-cli-mcp-auth-manual-exchange; ET-047
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

The manual MCP authorization command read a pasted code or full redirect URL with a normal buffered terminal read. In an interactive TTY, the terminal therefore echoed the complete redirect URL, including its authorization code and state, before AGH printed the otherwise-redacted status payload.

## Reproduction

1. Run `agh mcp auth login <name> --manual` in an interactive terminal.
2. Paste a full loopback redirect URL containing a valid authorization code and state.
3. Observe the terminal transcript before the final status payload.

**Expected:** The secret-class paste is hidden; only the prompt, a newline, and redacted status are visible.
**Actual:** The terminal echoed the complete pasted redirect URL.

## Evidence

- Red/green and sanitized live replay: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-manual-tty-redaction.json`.
- The pre-fix pseudo-terminal replay reproduced the complete echo. No sensitive value was copied into the durable QA artifact.

## Fix

- **Root cause:** `exchangeManualMCPAuth` always used `bufio.Reader.ReadString`, which leaves terminal echo enabled.
- **Correction:** Interactive terminal input now uses `term.ReadPassword` and emits only the post-input newline. Non-terminal stdin keeps the original line-oriented behavior for pipes and scripts.
- **Fix commit:** pending Phase D checkpoint
- **Regression test:** The canonical CLI auth suite proves the terminal branch returns the secret to the exchange owner without writing it, and proves piped input never invokes the terminal reader.

## Verification

- The new owner regression was RED before the production helper existed.
- Seven focused CLI cases pass under `-race`, covering the hidden terminal branch, the pipe branch, and daemon-owned manual exchange.
- A rebuilt isolated daemon accepted a full redirect URL through a real pseudo-terminal and returned `authenticated`, `token_present=true`, and `refreshable=true` without echoing the URL, code, or state.
