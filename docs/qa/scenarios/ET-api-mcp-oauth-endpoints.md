---
id: ET-api-mcp-oauth-endpoints
area: ET
title: Manage scoped MCP OAuth through daemon API routes
persona: Ada
journey: J-mcp-authorize-repair
expected: HTTP and UDS begin/exchange/logout return equivalent redacted contracts for an explicit global or workspace target; begin requires `mode=automatic|manual`, and manual creates a fresh paste-based PKCE session; HTTP mutations require loopback privilege; the HTTP-only callback completes on loopback while refusing non-loopback binds; replacing or deleting a server invalidates pending target/callback completion, preserves any prior token record, and never sends its bearer to the replacement endpoint. A successful exchange supersedes every older refresh generation.
entry_points: POST /api/settings/mcp-servers/{name}/auth/begin; POST /api/settings/mcp-servers/{name}/auth/exchange; POST /api/settings/mcp-servers/{name}/auth/logout; GET /api/mcp/oauth/callback
qa_status: blocked-verify
bug_ids: BUG-20260715-mcp-oauth-name-segment
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-non-loopback-callback.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/024-mcp-oauth-endpoints; /Users/pedronauck/dev/qa-labs/compozy-mcp-oauth-nonloopback-20260729-20260729-094415-836845-lab/qa-artifacts/qa/evidence/001-mcp-oauth-nonloopback; /Users/pedronauck/dev/qa-labs/compozy-mcp-oauth-replacement-20260729-20260729-095438-704553-lab/qa-artifacts/qa/evidence/001-mcp-replacement;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-047; ET-cli-mcp-auth-manual-exchange; ET-cli-mcp-authorize
---

Historical QA note: effective IPv4/IPv6 loopback callback origin and documented 503 outcome coverage remains pending.

Added by marketplace Task 04. QA should compare HTTP and UDS payloads, prove two homonymous
workspace servers cannot read each other's tokens, confirm logout removes only the selected scoped
credential, and scan responses, SSE, events, and logs for OAuth codes, redirect URLs, verifiers, and
tokens.

QA impact 2026-07-16: reset to untested after binding PKCE sessions and durable tokens to the exact
server definition. Exercise both sequential replacement and a mutation while token exchange is in
flight; verify the old token remains stored but is neither reported authenticated nor sent to the
replacement endpoint. Also leave an expired session abandoned, begin another target, and confirm the
expired state can no longer complete.

QA impact 2026-07-16: begin now derives its callback from the effective loopback listener instead of
assuming `127.0.0.1`; verify IPv4, IPv6, and non-loopback behavior plus the callback's documented
`503` HTML response when runtime state is unavailable.

QA impact 2026-07-17: begin now requires the explicit automatic/manual mode over HTTP and UDS.
Exercise a refresh that starts after begin but finishes after exchange; the exchanged credential must
remain authoritative. A wrong-state redirect must not consume the active session.

QA result 2026-07-29: HTTP/UDS begin, exchange, and logout; loopback automatic callback; wrong-state
non-consumption; target replacement and deletion; old-bearer withholding after restart; real expiry;
newer-exchange versus blocked-refresh ordering; non-loopback 403/503 behavior; redaction; and cleanup
all passed. The historical name-segment fix passed its fresh retest. The defensive callback-503
branch with an absent settings runtime remains Pending because no public healthy-daemon fault owner
can create that handler state.
