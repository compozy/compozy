---
id: ET-web-marketplace-mcp-authorize-installed
area: ET
title: Authorize an MCP server from Installed scope
persona: Bruno
journey: J-mcp-authorize-repair
expected: An installed OAuth MCP server that needs login exposes Authorize in the MCP Installed scope and detail, reuses the scoped daemon authorization flow, and reports success only after authenticated status and token presence are both confirmed.
entry_points: /marketplace/mcps?tab=installed; /marketplace/mcp/<entry-id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-mcp-authorize; ET-web-mcp-authorize-manual; ET-web-mcp-status-matrix
---

Added by the unified Marketplace hard cut. Cover global and active-workspace definitions without
exposing OAuth codes, tokens, PKCE verifiers, or bound secret references.

QA impact 2026-07-18: post-install authorization opens the exact Marketplace detail identity and
retains scope plus workspace even if the active workspace changes before the toast action is used.

QA impact 2026-07-18: Installed-card and detail authorization dialogs preserve the server's
effective global or workspace source instead of inheriting the collection scope.

QA impact 2026-07-18: the post-install toast action now opens the canonical singular detail route
`/marketplace/mcp/<entry-id>` with the exact install identity, scope, and workspace query state.

QA impact 2026-07-18: when two installed definitions share one `catalog_entry`, detail status and
authorization resolve the exact `installed_name` before catalog identity. Verify the other install
cannot supply the displayed runtime/auth state or receive the authorization request.

QA impact 2026-07-19: while installed-detail OAuth authorization is awaiting confirmation, the
workspace- or global-scoped MCP projection polls at the dedicated authorization cadence and reports
success only after the refreshed status is authenticated with a token present.
