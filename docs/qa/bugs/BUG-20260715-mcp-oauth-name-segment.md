# BUG-20260715-mcp-oauth-name-segment: OAuth token persistence rejects MCP names with spaces

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Personas Affected:** Bruno; Iris; Ada
- **Journey Step:** J-mcp-authorize-repair, OAuth callback or manual exchange
- **Scenarios:** ET-web-mcp-authorize; ET-web-mcp-authorize-manual; ET-cli-mcp-auth-manual-exchange; ET-api-mcp-oauth-endpoints; MS-029
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 isolated Marketplace QA

## Summary

MCP server names are public human-readable identities and may contain spaces. OAuth completed at the authorization server, but token persistence failed when the exact server name was inserted into a Vault owner path segment. The callback returned HTTP 400, so Web and manual CLI authorization could never complete for a valid name such as `QA OAuth MCP`.

## Reproduction

1. Install an OAuth-protected MCP server named `QA OAuth MCP` in a workspace.
2. Begin authorization and complete the PKCE callback, or submit the code through manual exchange.
3. Observe the callback result and scoped auth status.

**Expected:** The exact server name remains the config/runtime/database identity, its token is stored under a safe opaque Vault owner segment, and status becomes `authenticated` with `token_present=true`.
**Actual:** Token persistence failed while constructing the Vault ref, the callback returned HTTP 400, and status remained `needs_login` with no token.

## Evidence

- Pre-fix browser callback reproduced the HTTP 400 after the fake authorization server completed PKCE.
- The structured manual exchange exposed the owning error without revealing token material: `vault: unsupported secret ref: [REDACTED] OAuth MCP/value`.
- Isolated red/green and live-replay record: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-oauth-name-segment.json`.
- Confirmed Web state: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/mcp-oauth-confirmed.png`.

## Fix

- **Root cause:** `vault.MCPSecretOwnerPrefix` validated workspace IDs as Vault path segments but used the MCP server name literally, despite the wider public name grammar.
- **Correction:** `MCPServerSegment` now preserves already-safe names and collision-safely hex-encodes unsafe or reserved-prefix names. Only the Vault owner segment changes; the exact public/config/database server identity remains unchanged.
- **Fix commit:** pending Phase D checkpoint
- **Regression tests:** The canonical Vault prefix suite proves unsafe-name encoding and reserved-prefix non-collision. The canonical GlobalDB auth-token suite proves scoped save/read isolation for `QA OAuth MCP`.

## Verification

- The focused regressions failed before the production change and pass afterward.
- Complete `internal/vault`, `internal/store/globaldb`, and `internal/settings` owners pass 927 race-enabled tests.
- A rebuilt isolated daemon completed the unchanged Web PKCE callback with `Authorization complete`.
- Scoped CLI status reports `authenticated`, `token_present=true`, and `refreshable=true` for the exact name `QA OAuth MCP`.
- The QA artifacts/runtime redaction scan found no fixture authorization code, access token, or refresh token outside fixture-owned inputs and the binary database.
