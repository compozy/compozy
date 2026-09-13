---
id: ET-web-marketplace-mcp-authorize-installed
area: ET
title: Authorize an extension MCP server from Installed, detail, and Settings
persona: Bruno
journey: J-mcp-authorize-repair
expected: An extension MCP server exposes its own status, runtime name and owner-qualified authorization in Installed, detail and Settings. A same-name manual definition remains independent. Confirmation requires authenticated status and token presence for the selected owner and scope.
entry_points: /marketplace/installed; /marketplace/<entry-id>; /settings/mcp
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/sentry-owned-auth.json
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps: ET-web-mcp-authorize; ET-web-mcp-authorize-manual; ET-web-mcp-status-matrix
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


## Current walk — Marketplace catalog hardcut

Owner: marketplace-catalog final tasks09/10. UI integration checks are not this live walk.
The historical routes and evidence below do not verify the current extension-only Marketplace.

1. Configure a manual MCP server and install an OAuth extension declaring the same logical server name.
   In Settings → MCP servers, confirm both rows remain distinguishable by owner and runtime name.
2. Open Marketplace → Installed. Confirm Needs authorization comes from the extension's MCP status.
   Click its Authorize action and complete the fixture provider flow. Confirm only that definition
   becomes authenticated with a token; the manual definition's credentials remain unchanged.
3. Repeat authorization from detail → Server and Settings, including a workspace/profile definition.
   Change the active workspace while the dialog is open; polling and completion must retain the
   original owner and scope. No tokens, codes, PKCE verifiers or secret references appear in status UI.
4. Open Edit configuration from each surface. Confirm only the applicable env/headers/endpoint
   override is editable. Save, reopen, reset and verify the package declaration and manual definition
   remain unchanged. Settings must not offer manual removal for an extension-provided definition.
5. Inspect an uninstalled MCP-backed entry: show declared Launch, Auth, Inputs, Scope and Owner,
   without runtime/status observations or authorization/edit controls. An entry with no MCP server
   has no Server section. Missing required inputs take priority over an authorization prompt.

## Historical evidence — superseded by the current walk

Previous report: docs/qa/reports/2026-08-13-pr-368-coderabbit.md

Previous evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-368-coderabbit-20260813-051821-831054-lab/qa-artifacts/qa/screenshots/mcp-global-install-clean.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-guided-linear-installed.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/mcp-status-after-linear.json

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

QA result 2026-07-30: the installed Linear surface exposed the exact target's authorization handoff
and remained unauthenticated. Human consent and the post-exchange token-present confirmation remain
`blocked-verify`.

2026-08-12 qa-impact: MCP install destination is derived from the menubar Global switch; `config_scope` was deleted. Reset to untested.

2026-08-12 walk: blocked-verify. This implementation cycle captured Storybook visual-contract evidence (`.compozy/tasks/global-workspace-menubar/evidence/visual/menubar-toggle/VC-01`–`VC-04`) and unit/typecheck coverage. An isolated QA lab with a live daemon (`COMPOZY_HOME`, production-parity web) was not started, so a persona walk through public entry points could not meet the qa-execution evidence standard.

2026-08-13 re-walk: the live Global marketplace opened an Airtable install with no prior credential state and the exact destination statement "Installs at Global — available in every workspace." Real OAuth consent and token-presence confirmation remain blocked-verify.
