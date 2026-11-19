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
agree row for row. A save that needs a daemon restart shows the same restart notice
and **Restart CompozyOS** action in the MCPs view as in Settings. Targeted verification: the Marketplace component suite and the
`McpServers`/`McpServersSearch` Storybook stories rendered the nine-server matrix in Chromium; a live
walk remains outstanding.

QA impact 2026-11-14 (MCP hot activation spec): The MCP body now stacks a CompozyOS group (with an approval cell: Waiting for approval · Added by a file in this project / Changed since you allowed it, plus Allow) above one read-only group per runtime, with a runtime filter. A waiting server has no runtime status (never started or probed). Re-walk the matrix with a waiting project server and with runtime groups present (see ET-runtime-mcp-inventory-copy).

QA impact 2026-11-16 (#1772): passive `/api/status` must not initialize a configured MCP server.
In an isolated synthetic lab, read status cold, run an explicit diagnostic, then read status warm
and after 30 seconds. Require `unknown/skipped` cold or stale, the observed state while fresh,
zero additional network calls from passive reads, live consent/auth/dead-state precedence, and
invalidation after definition, credential or extension lifecycle changes. Settings and doctor
retain their active probes. Verify exact session state/badge totals and ten most recent failures,
including archived history, with a large catalog and live overrides.

Targeted retest 2026-11-16 (#1772): the final binary passed an isolated real-daemon walk
with 3,181 synthetic sessions and a counting HTTP MCP server. Three cold status reads
preserved all state/badge counts and 637 failures while making zero MCP requests. Doctor
made two protocol requests; the following passive read reused `ready/succeeded` without
network traffic. After the 30-second observation window, status returned
`unknown/skipped/stale`, again without traffic. Canonical daemon, session, globaldb and
observe suites passed consent, identity invalidation, live overrides, archived failures
and the ten-recent bound. The isolated lab teardown reported `TEARDOWN_ALL_CLEAN=true`.
This focused walk does not replace the unrelated OAuth/vendor branches of the matrix.

## Explicit check

After integrating the MCP redesign, select Check again for one broken server after repairing it.
`POST /api/settings/mcp-servers/{name}/check` uses the row's exact `scope`, `workspace_id`, `profile`,
and `owner`, probes only that server, and returns its item with refreshed `runtime_status`.
A waiting definition remains unprobed; app tokens cannot call this mutation. A successful probe
clears a durable dead mark. Verify a colliding logical name under another owner is unaffected.
Focused real MCP transport coverage lives in `TestSettingsRuntimeSurfaceMCPServerRuntimeStatus`;
the integrated UI walk remains pending until the frontend slices are integrated.

QA impact 2026-11-16 (MCP redesign S5): the Settings list layout is unchanged. A row whose status is
destructive (Setup problem, Unavailable, Couldn't connect, Access denied…) and has no Sign in /
Reauthorize repair gets a quiet "Not working ›" action. It opens "<server> isn't working" beside
the button (Settings variant: no "Open in Settings"), with the runtime diagnostic under "What the
server said". Check again posts the exact-scope check, replaces the row in the list cache with the
returned item; a server that works closes the panel and its row reads its new status, one that still
fails keeps the panel open with "Still not working after the check."

QA impact 2026-11-17 (#1794, daemon CPU hot paths):
HTTP detail summaries reuse the daemon read model with the same payload. Auth and runtime revisions fence reuse; unsignaled external health is revalidated at the configured poll cadence. The targeted run exercises HTTP payloads and hosted transport; it does not claim this complete browser matrix.
See [targeted runtime evidence](../reports/2026-11-17-daemon-cpu-hot-paths.md).

QA impact 2026-11-18 (#1828): for a remote OAuth server with an expired access token and a usable
CIMD registration, Settings receives `refreshable=true` even for registrations saved without
the auth method. The server keeps its tools and does not offer **Sign in** solely because of
expiry. A runtime `auth_refresh_failed` result continues to offer **Reauthorize**.
Follow the refresh steps in [ET-api-mcp-oauth-endpoints](ET-api-mcp-oauth-endpoints.md).

QA impact 2026-11-18 (#1830, #1831): with a local fixture, probe during the early renewal window
and return a transient refresh failure. Expect **Ready** while the bound access token still works.
With an expired access token and an unusable persisted registration, expect **Sign-in expired**
and **Reauthorize**. Discovery should not contact the OAuth issuer again until the local
credentials allow renewal. These cases reuse the existing status and repair controls.

QA impact 2026-11-18 ([#1850](https://github.com/compozy/compozy/issues/1850))

With one slow fixture, verify the session MCP control shows its discovery reason while other rows
remain ready. Use Check again in Settings and confirm the targeted runtime diagnostic reports the
latest attempt.

For #1850, turn one fixture server off using its Settings switch. Reload Settings and verify
the row and its definition are still present, the switch is unchecked, and the neutral **Off**
status replaces Ready. The settings payload reports `off` with `probe: "skipped"`. Re-enable it
and verify discovery resumes while the second fixture's state stays unchanged.
See the [off-switch walk](../reports/2026-11-18-mcp-server-off-switch.md).
