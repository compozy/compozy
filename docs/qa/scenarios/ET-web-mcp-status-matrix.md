---
id: ET-web-mcp-status-matrix
area: ET
title: MCP management status matrix renders daemon truth
persona: Bruno
journey: J-mcp-authorize-repair
expected: The `/marketplace/mcps` cards and detail management render daemon-owned auth, runtime, probe, and negotiated protocol version with truthful success/warning/danger/neutral tones. Failed, unavailable, denied, absent, and unknown runtime states are never labeled running. `probe=skipped` is never treated as a failure. Sign in/Reauthorize is offered to unauthenticated OAuth HTTP remotes (or auth_refresh_failed), including bare header-free HTTP remotes reporting auth_required. Open remotes, stdio, and header-auth servers without explicit OAuth never get browser authorization.
entry_points: web `/marketplace?kind=mcps` (Marketplace › MCPs); Settings › MCP servers; `GET /api/settings/mcp-servers`
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/mcp-installed-default.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/web-status-matrix.json
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: MS-029
---

Passed in the 2026-07-30 final rerun: GitHub rendered dead with its probe skipped and never appeared
green; Linear rendered authorization-required without being labeled running. The settings payload
contained neither tokens nor Vault refs. The authenticated-remote 401 branch remains unit-covered
but was not newly induced against a vendor endpoint in this walk.

Planning note: workspace-guard action-gating verification remains pending; no bug fix is associated with this scenario.

story: As a builder I read every configured MCP server's real configuration, authorization, runtime, and probe status at a glance without a plausible green masking a broken server.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

QA impact 2026-07-15: new behavior from Task 08 (ADR-006). The `/mcp` page replaced the hardcoded success dot with the composed status matrix. Flagged untested for the next QA cycle.

QA impact 2026-07-16: the Add server action is now absent while the workspace guard cannot
resolve a valid scope, eliminating the previous visible-but-inert control.

QA impact 2026-07-18: the consolidated Installed card now derives its label and tone from the
canonical settings status model instead of defaulting non-auth states to `running`.

QA impact 2026-07-18: OAuth authorization now targets `source_metadata.effective_source`; verify a
global-only MCP remains authorizable when it appears in a workspace-scoped collection response.

QA impact 2026-07-30 deep-review remediation: reset after authenticated remote HTTP 401 responses
were reclassified from unreachable to `mcp_auth_required`. Verify cards and detail management offer
Authorize/Reauthorize without claiming the server is running, while transport/network failures remain
unavailable and no token, binding ref, or OAuth redirect appears in status payloads.

QA impact 2026-09-28 (ui-normie-pass, settings slice): Settings › MCP servers rows now show one worst-of Status pill (for example "Needs sign-in", "Unavailable", "Ready"); the per-signal setup/sign-in/connection/tools breakdown with its raw detail lines moved into the selection strip opened by the server name. The no-false-green rule is unchanged. Flagged for the next QA cycle.

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

QA impact 2026-11-04 (issue #1305 gap 3): the Marketplace MCPs view is back as `/marketplace?kind=mcps`,
reached through the **Plugins | MCPs** toggle beside the Marketplace search. It renders the same MCP
server table, status cells, and per-row Authorize/Reauthorize (or Sign in) and edit actions as
Settings › MCP servers, from the same `GET /api/settings/mcp-servers` read; the retired
`/marketplace/mcps` card path stays not-found. Walk the matrix in both entry points and confirm they
agree row for row. Targeted verification: the Marketplace component suite and the
`McpServers`/`McpServersSearch` Storybook stories rendered the nine-server matrix in Chromium; a live
walk remains outstanding.
