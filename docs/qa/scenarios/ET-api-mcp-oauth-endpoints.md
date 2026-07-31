---
id: ET-api-mcp-oauth-endpoints
area: ET
title: Manage scoped MCP OAuth through daemon API routes
persona: Ada
journey: J-mcp-authorize-repair
expected: HTTP and UDS begin/exchange/logout return equivalent redacted contracts for an explicit global or workspace target; begin requires `mode=automatic|manual`, resolves pre-registration then CIMD then one DCR fallback, and manual accepts only a full redirect URL for a fresh PKCE session. The lifecycle binds target/resource/issuer/redirect/scopes/fingerprint, validates state and RFC 9207 issuer, burns every exchange attempt, and preserves any prior token on failure. HTTP mutations require loopback privilege; the HTTP-only callback completes on loopback while refusing non-loopback binds; replacing or deleting a server invalidates pending target/callback completion and never sends its bearer to the replacement endpoint.
entry_points: POST /api/settings/mcp-servers/{name}/auth/begin; POST /api/settings/mcp-servers/{name}/auth/exchange; POST /api/settings/mcp-servers/{name}/auth/logout; GET /api/mcp/oauth/callback
qa_status: blocked-verify
bug_ids: BUG-20260715-mcp-oauth-name-segment
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/cli-mcp-install-linear.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/mcp-status-after-linear.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-guided-linear-installed.png
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-047; ET-cli-mcp-auth-manual-exchange; ET-cli-mcp-authorize
---

Historical QA note: configured IPv4/IPv6 loopback callback-origin coverage remains pending.

Added by marketplace Task 04. QA should compare HTTP and UDS payloads, prove two homonymous
workspace servers cannot read each other's tokens, confirm logout removes only the selected scoped
credential, and scan responses, SSE, events, and logs for OAuth codes, redirect URLs, verifiers, and
tokens.

QA impact 2026-07-16: reset to untested after binding PKCE sessions and durable tokens to the exact
server definition. Exercise both sequential replacement and a mutation while token exchange is in
flight; verify the old token remains stored but is neither reported authenticated nor sent to the
replacement endpoint. Also leave an expired session abandoned, begin another target, and confirm the
expired state can no longer complete.

QA impact 2026-07-16 (superseded 2026-07-30): callback origin had briefly followed the effective
listener. The MCP 2026 hard cut now treats `mcp.oauth.redirect_uri` as authoritative and never derives
it from the daemon listener.

QA impact 2026-07-17: begin now requires the explicit automatic/manual mode over HTTP and UDS.
Exercise a refresh that starts after begin but finishes after exchange; the exchanged credential must
remain authoritative. A wrong-state redirect must not consume the active session.

QA result 2026-07-29: HTTP/UDS begin, exchange, and logout; loopback automatic callback; wrong-state
non-consumption; target replacement and deletion; old-bearer withholding after restart; real expiry;
newer-exchange versus blocked-refresh ordering; non-loopback 403/503 behavior; redaction; and cleanup
all passed. The historical name-segment fix passed its fresh retest. The defensive callback-503
branch with an absent settings runtime remains Pending because no public healthy-daemon fault owner
can create that handler state.

QA impact 2026-07-30 deep-review remediation: re-walk configured loopback callback validation,
RFC 9207 `iss`, DCR token-endpoint auth-method persistence, resource indicators, catalog-egress
policy, refresh, and logout. Full vendor consent remains `blocked-verify`; local fixture branches must
still pass without exposing codes, redirects, verifiers, registration tokens, or bearer tokens.

QA result 2026-07-30: deterministic installation and status remained redacted and returned
`next_step=authorize`; no vendor consent, code exchange, refresh token, or logout was performed.
The scenario therefore remains `blocked-verify` only for human-owned Linear consent.
