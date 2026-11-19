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

Gap 1 rule (issue #1305): configure a bare HTTP remote with no `auth`, `headers`, or
`secret_headers` against an OAuth PRM / WWW-Authenticate challenge. Settings must offer
**Sign in** through the existing authorization dialog. Complete a fresh manual redirect,
then confirm `authenticated && token_present` through a fresh status read and a successful
runtime tool probe. Reload Settings to verify persistence. An open HTTP remote and remotes
with plain or secret headers and no explicit OAuth policy must never receive this action. In Advanced editor mode,
**Use OAuth** must toggle the saved `auth.registration=auto` block; editing a header server
must preserve its headers without returning their values.

Gap 1 retest 2026-11-05: the changed bare-remote, header exclusion, header preservation,
and OAuth-toggle branches passed in the isolated daemon lab. See
`docs/qa/reports/2026-11-05-issue-1305-gap1.md`. This does not replace historical coverage or
vendor-consent blockers for unrelated branches in this scenario.

Issue #1334 regression: open Claude and Codex sessions before adding a user-scope HTTP MCP
server backed by an OAuth stub. Complete manual PKCE authorization through the public API and
require each same session to discover and call a real stub tool. Start another session after
Ready and require the same call. Log out and require the old sessions to lose callable access;
a subsequent login restores it, while deleting or disabling the source withdraws it again.
Record logical and native session IDs, provider reloads, successful stub calls and redacted status.
Renaming a source updates root delegation budgets at the next turn without widening a child or
instance allowlist. A failed full hosted projection must fail readiness instead of claiming success.

QA impact 2026-11-14 (MCP hot activation spec): `auth/begin` gains an optional `definition_digest`. For a workspace record waiting for approval: without it `409 mcp_server_waiting_approval` and no network request; with the current digest the approval is recorded, then sign-in begins; with a stale digest `409 mcp_approval_definition_changed`. Retained OAuth clients and redirects recheck consent. Passive auth status stays readable. Re-walk begin/exchange on a waiting project server.

QA impact 2026-11-14 review fixes: sign out after revoking project approval. Local credentials and pending auth state must be removed while approval is waiting, with no request to the MCP/OAuth server. The focused daemon auth suite exercises the real token store and a request-counting HTTP fixture; it does not replace the full live-provider walk.

QA impact 2026-11-17 (#1785): in an isolated home with a local OAuth authority, expire a stored
access token while retaining a usable refresh token. Status remains `expired`, `token_present=true`,
`refreshable=true`; discovery/call renews it without sign-in. Concurrent calls share one refresh.
Return `invalid_grant` or `revoked`: require `needs_login`, no stored token and withdrawn tools.
Return a transient error: credentials and advertised tools remain; requests during retry backoff
send no additional OAuth traffic, and a later successful attempt recovers. Capture refresh logs
with server, cause class and attempt count, and verify access/refresh tokens, codes, verifiers and
client secrets are absent. An expired dynamic-client secret is not a usable refresh path.

QA impact 2026-11-18 (#1828): authorize through a valid HTTPS client ID metadata document
(CIMD), expire the access token and check `compozy mcp auth status` or the auth status route.
Expect `expired`, `token_present=true`, and `refreshable=true`. Discover tools and confirm
the authorization server receives a refresh grant with the same document URL as `client_id`
and no client secret. Status returns to `authenticated` and tools are available. Repeat with
a pre-upgrade registration whose auth method is empty. Expect the same result with no callback
or new registration. Verify that an omitted `scope` on refresh preserves the granted scopes in
status, and that changing the configured CIMD redirect URI disables renewal until reauthorization.
The isolated SQLite and HTTP path is covered by
`TestSettingsRuntimeSurfaceMCPAuthAllowsOperatorLoopback`.

QA impact 2026-11-18 (#1829, #1830, #1831), using only local OAuth and MCP fixtures:

- Advertise `offline_access` at the issuer and authorize a refresh-capable client with a resource
  scope. Verify both scopes and the issuer's existing consent parameters in the authorization
  request. Repeat without issuer support and with a response that grants access without a refresh
  token. On a bare remote, verify the Bearer challenge's scopes take precedence over protected
  resource metadata. With neither source available, verify the request leaves the issuer's
  defaults in effect.
  Reuse a registration that did not register offline access and confirm it keeps its original
  scopes. When the issuer declines a required resource scope at registration, expect an error
  before browser handoff and retention of the existing registration. Auth status shows the
  result through the existing redacted fields.
- Expire the persisted registration's client secret. Observe `refreshable=false`, an instruction to
  sign in again, and a retained token. Repeat discovery across the retry interval and confirm
  there is no OAuth traffic. Repair the registration and confirm discovery renews with the same
  grant. Inspect status and logs for absence of credentials and Vault references.
- Advance a fixture clock into the final 10% of the access token lifetime, up to 30 seconds before
  expiry. Passive auth status still reports `authenticated` with the original expiry. Discovery
  starts one renewal. On a transient failure, discovery still uses the existing bearer until
  expiry and respects backoff. For an access-only response, discovery works through the same
  window without attempting renewal.
- Before expiry, have an automatic registration discover a replacement issuer. Expect the valid
  bearer to remain bound to the original resource and the refresh grant to be withdrawn.
  The new issuer must receive no token request. After expiry, further status or discovery checks
  cannot use the old bearer and send no more renewal traffic.

The OAuth adapter and executor suites cover these fixture walks.
